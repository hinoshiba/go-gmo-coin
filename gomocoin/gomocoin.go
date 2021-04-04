package gomocoin

import (
	"fmt"
	"sync"
	"time"
	"context"
	"strconv"
	"encoding/json"
)

const (
	STATUS_MAINTENANCE string = "MAINTENANCE"
	STATUS_PREOPEN string = "PREOPEN"
	STATUS_OPEN string = "OPEN"

	SYMBOL_BTC string = "BTC"
	SYMBOL_ETH string = "ETH"
	SYMBOL_BCH string = "BCH"
	SYMBOL_LTC string = "LTC"
	SYMBOL_XRP string = "XRP"
	SYMBOL_BTC_JPY string = "BTC_JPY"
	SYMBOL_ETH_JPY string = "ETH_JPY"
	SYMBOL_BCH_JPY string = "BCH_JPY"
	SYMBOL_LTC_JPY string = "LTC_JPY"
	SYMBOL_XRP_JPY string = "XRP_JPY"

	SIDE_BUY  string = "BUY"
	SIDE_SELL string = "SELL"

	TYPE_EXECUTION_STREAM string = "MARKET"
	TYPE_SETTLE_FIX string = "CLOSE"

	FMT_TIME string = "2006-01-02T15:04:05.000Z"
)

type GoMOcoin struct {
	client      *Client

	rate        map[string]*RateData

	ctx_cancel  context.CancelFunc
	mtx         *sync.Mutex
}

func NewGoMOcoin(api_key string, secret_key string, b_ctx context.Context) (*GoMOcoin, error) {
	ctx, ctx_cancel := context.WithCancel(b_ctx)
	auth := NewAuth(api_key, secret_key)
	clnt := NewClient(auth)
	clnt.RunPool(ctx)

	rate := make(map[string]*RateData)
	self := &GoMOcoin{client:clnt, ctx_cancel:ctx_cancel, rate:rate, mtx:new(sync.Mutex)}
	ok, err := self.checkStatus()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("store not open.")
	}
	return self, nil
}

func (self *GoMOcoin) GetPositions(symbol string) ([]*Position, error) {
	self.lock()
	defer self.unlock()

	param := "symbol=" + symbol
	ret, err := self.request2Pool("GET", "/private", "/v1/openPositions", param, nil)
	if err != nil {
		return nil, err
	}

	var p posBase
	if err := json.Unmarshal(ret, &p); err != nil {
		return nil, err
	}
	if p.Status != 0 {
		return nil, fmt.Errorf("%s", p.Message())
	}

	return p.Data.Poss, nil
}

func (self *GoMOcoin) GetFixes(symbol string) ([]*Fix, error) {
	self.lock()
	defer self.unlock()

	param := "symbol=" + symbol + "&page=1&count=100"
	ret, err := self.request2Pool("GET", "/private", "/v1/latestExecutions", param, nil)
	if err != nil {
		return nil, err
	}

	var fb fixBase
	if err := json.Unmarshal(ret, &fb); err != nil {
		return nil, err
	}
	if fb.Status != 0 {
		return nil, fmt.Errorf("%s", fb.Message())
	}

	return fb.Data.Fixes, nil
}

func (self *GoMOcoin) OrderStreamIn(mode string, symbol string, size float64) error {
	size_str := strconv.FormatFloat(size, 'f', -1, 64)
	order := (`{"symbol" : "` + symbol + `", "side" : "` + mode + `",
			"executionType" : "` + TYPE_EXECUTION_STREAM + `",
			"size" : "` + size_str + `"}`)

	ret, err := self.request2Pool("POST", "/private", "/v1/order", "", []byte(order))
	if err != nil {
		return err
	}
	var r respBase
	if err := json.Unmarshal(ret, &r); err != nil {
		return err
	}
	if r.Status != 0 {
		return fmt.Errorf("%s", r.Message())
	}
	return nil
}

func (self *GoMOcoin) OrderStreamOut(pos *Position) error {
	var mode string
	if pos.RawMode == SIDE_SELL {
		mode = SIDE_BUY
	}
	if pos.RawMode == SIDE_BUY {
		mode = SIDE_SELL
	}
	if mode == "" {
		return fmt.Errorf("have a unkown mode. '%s'", pos.RawMode)
	}

	order := (`{"symbol" : "` + pos.RawSymbol + `", "side" : "` + mode + `",
			"executionType" : "` + TYPE_EXECUTION_STREAM + `",
			"settlePosition":[
				{"size" : "` + pos.RawSize + `", "positionId" : ` + pos.Id() + `}
			]}`)

	ret, err := self.request2Pool("POST", "/private", "/v1/closeOrder", "", []byte(order))
	if err != nil {
		return err
	}
	var r respBase
	if err := json.Unmarshal(ret, &r); err != nil {
		return err
	}
	if r.Status != 0 {
		return fmt.Errorf("%s", r.Message())
	}
	return nil
}

func (self *GoMOcoin) lock() {
	self.mtx.Lock()
}

func (self *GoMOcoin) unlock() {
	self.mtx.Unlock()
}

func (self *GoMOcoin) Close() error {
	self.ctx_cancel()
	return nil
}

func (self *GoMOcoin) request2Pool(method string, base_path string, path string, param string, body []byte) ([]byte, error) {
	req, err := self.client.NewRequest(method, base_path, path, param, body)
	if err != nil {
		return nil, err
	}
	return self.client.PostPool(req)
}

type apibase struct {
	Status       int        `json:"status"`
	Responsetime string     `json:"responsetime"`
	Messages     []*Message `json:"messages"`
}

func (self *apibase) Message() string {
	var ms string
	for _, m := range self.Messages {
		ms += m.ToString() + ", "
	}

	return ms
}

type Message struct {
	Code   string `json:"message_code"`
	Msg    string `json:"message_string"`
}

func (self *Message) ToString() string {
	return fmt.Sprintf("%s : %s", self.Code, self.Msg)
}

type statusBase struct {
	apibase
	Data statusData `json:"data"`
}

type statusData struct {
	Status string `json:"status"`
}

func (self *GoMOcoin) checkStatus() (bool, error) {
	ret, err := self.request2Pool("GET", "/public", "/v1/status", "", nil)
	if err != nil {
		return false, err
	}
	var s *statusBase
	if err := json.Unmarshal(ret, &s); err != nil {
		return false, err
	}

	if s.Data.Status != STATUS_OPEN {
		return false, nil
	}
	return true, nil
}

type fixBase struct {
	apibase
	Data struct {
		Fixes []*Fix `json:"list"`
	} `json:"data"`
}

type Fix struct {
	RawRelationId int    `json:"orderId"`
	RawId         int    `json:"executionId"`
	RawSymbol     string `json:"symbol"`
	RawMode       string `json:"side"`
	RawSettlement string `json:"settleType"`
	RawSize       string `json:"size"`
	RawPrice      string `json:"price"`
	RawYield      string `json:"lossGain"`
	RawDate       string `json:"timestamp"`
}

func (self *Fix) Id() string {
	return fmt.Sprintf("%v", self.RawId)
}

func (self *Fix) Symbol() string {
	return self.RawSymbol
}

func (self *Fix) OrderType() string {
	return self.RawMode
}

func (self *Fix) Size() float64 {
	size, err := strconv.ParseFloat(self.RawSize, 64)
	if err != nil {
		return float64(-1)
	}
	return size
}

func (self *Fix) Price() float64 {
	price, err := strconv.ParseFloat(self.RawPrice, 64)
	if err != nil {
		return float64(-1)
	}
	return price
}

func (self *Fix) Yield() (float64, error) {
	yield, err := strconv.ParseFloat(self.RawYield, 64)
	if err != nil {
		return float64(-1), err
	}
	return yield, nil
}

func (self *Fix) Date() (time.Time, error) {
	t, err := time.Parse(FMT_TIME, self.RawDate)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

type posBase struct {
	apibase
	Data struct {
		Poss []*Position `json:"list"`
	} `json:"data"`
}

type Position struct {
	RawId     int    `json:"positionId"`
	RawMode   string `json:"side"`
	RawSymbol string `json:"symbol"`
	RawSize   string `json:"size"`
	RawPrice  string `json:"price"`
}

func (self *Position) Id() string {
	return fmt.Sprintf("%v", self.RawId)
}

func (self *Position) Symbol() string {
	return self.RawSymbol
}

func (self *Position) Size() float64 {
	size, err := strconv.ParseFloat(self.RawSize, 64)
	if err != nil {
		return float64(-1)
	}
	return size
}

func (self *Position) Price() float64 {
	price, err := strconv.ParseFloat(self.RawPrice, 64)
	if err != nil {
		return float64(-1)
	}
	return price
}

func (self *Position) OrderType() string {
	return self.RawMode
}

type rateBase struct {
	apibase
	Data []*RateData `json:"data"`
}

type RateData struct {
	Ask       string `json:"ask"`
	Bid       string `json:"bid"`
	High      string `json:"high"`
	Last      string `json:"last"`
	Low       string `json:"low"`
	Symbol    string `json:"symbol"`
	Timestamp string `json:"timestamp"`
	Volume    string `json:"volume"`
}

type Rate struct {
	ask    float64
	bid    float64
	high   float64
	last   float64
	low    float64
	symbol string
	t      time.Time
	volume float64
}

func (self *Rate) Ask() float64 {
	return self.ask
}

func (self *Rate) Bid() float64 {
	return self.bid
}

func (self *Rate) High() float64 {
	return self.high
}

func (self *Rate) Last() float64 {
	return self.last
}

func (self *Rate) Low() float64 {
	return self.low
}

func (self *Rate) Symbol() string {
	return self.symbol
}

func (self *Rate) Time() time.Time {
	return self.t
}

func (self *Rate) Volume() float64 {
	return self.volume
}

func conv2Rate(rd *RateData) (*Rate, error) {
	ask, err := strconv.ParseFloat(rd.Ask, 64)
	if err != nil {
		return nil, err
	}
	bid, err := strconv.ParseFloat(rd.Bid, 64)
	if err != nil {
		return nil, err
	}
	high, err := strconv.ParseFloat(rd.High, 64)
	if err != nil {
		return nil, err
	}
	last, err := strconv.ParseFloat(rd.Last, 64)
	if err != nil {
		return nil, err
	}
	low, err := strconv.ParseFloat(rd.Low, 64)
	if err != nil {
		return nil, err
	}
	volume, err := strconv.ParseFloat(rd.Volume, 64)
	if err != nil {
		return nil, err
	}

	t, err := time.Parse(FMT_TIME, rd.Timestamp)
	if err != nil {
		return nil, err
	}

	return &Rate{
		ask: ask,
		bid: bid,
		high: high,
		last: last,
		low: low,
		symbol: rd.Symbol,
		t: t,
		volume: volume,
	}, nil
}

func (self *GoMOcoin) GetRate() (map[string]*Rate, error) {
	self.lock()
	defer self.unlock()

	ret, err := self.request2Pool("GET", "/public", "/v1/ticker", "", nil)
	if err != nil {
		return nil, err
	}
	var rb rateBase
	if err := json.Unmarshal(ret, &rb); err != nil {
		return nil, err
	}

	rs := make(map[string]*Rate)
	for _, rd := range rb.Data {
		r, err := conv2Rate(rd)
		if err != nil {
			return nil, err
		}

		rs[rd.Symbol] = r
		self.rate[rd.Symbol] = rd
	}

	return rs, nil
}

type respBase struct {
	apibase
	Data string `json:"data"`
}
