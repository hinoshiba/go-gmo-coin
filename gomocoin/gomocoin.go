package gomocoin

import (
	"fmt"
	"sync"
	"time"
	"context"
	"strconv"
	"encoding/json"
)

import (
	"github.com/vouquet/shop"
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

func (self *GoMOcoin) GetPositions(symbol string) ([]shop.Position, error) {
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

	poss := []shop.Position{}
	for _, pos := range p.Data.Poss {
		poss = append(poss, pos)
	}
	return poss, nil
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
	var o orderBase
	if err := json.Unmarshal(ret, &o); err != nil {
		return err
	}
	if o.Status != 0 {
		return fmt.Errorf("%s", o.Message())
	}
	return nil
}

func (self *GoMOcoin) OrderStreamOut(pos shop.Position) error {
	pb, ok := pos.(*Position)
	if !ok {
		return fmt.Errorf("unkown type at this store.")
	}
	var mode string
	if pb.RawMode == SIDE_SELL {
		mode = SIDE_BUY
	}
	if pb.RawMode == SIDE_BUY {
		mode = SIDE_SELL
	}
	if mode == "" {
		return fmt.Errorf("have a unkown mode. '%s'", pb.RawMode)
	}

	order := (`{"symbol" : "` + pb.RawSymbol + `", "side" : "` + mode + `",
			"executionType" : "` + TYPE_EXECUTION_STREAM + `",
			"settlePosition":[
				{"size" : "` + pb.RawSize + `", "positionId" : ` + pb.Id() + `}
			]}`)

	ret, err := self.request2Pool("POST", "/private", "/v1/closeOrder", "", []byte(order))
	if err != nil {
		return err
	}
	var o orderBase
	if err := json.Unmarshal(ret, &o); err != nil {
		return err
	}
	if o.Status != 0 {
		return fmt.Errorf("%s", o.Message())
	}
	return nil
}

func (self *GoMOcoin) lock() {
	self.mtx.Lock()
}

func (self *GoMOcoin) unlock() {
	self.mtx.Unlock()
}

func (self *GoMOcoin) Close() {
	self.ctx_cancel()
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

func (self *GoMOcoin) GetRate() (map[string]shop.Rate, error) {
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

	rs := make(map[string]shop.Rate)
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

type orderBase struct {
	apibase
	Data string `json:"data"`
}

func (self *GoMOcoin) Order(mode string, symbol string, size float64, r shop.Rate) (string, error) {
	var o_price float64 = 0
	var o_rate  shop.Rate = r

	if r == nil {
		rate_data, ok := self.rate[symbol]
		if !ok {
			return "", fmt.Errorf("undefined symbol. %s", symbol)
		}

		rate, err := conv2Rate(rate_data)
		if err != nil {
			return "", err
		}

		o_rate = rate
	}
	if mode == SIDE_SELL {
		o_price = o_rate.Bid()
	}
	if mode == SIDE_BUY {
		o_price = o_rate.Ask()
	}
	price_str := fmt.Sprintf("%f", o_price)
	size_str := strconv.FormatFloat(size, 'f', -1, 64)

	order := (`{"symbol" : "` + symbol + `", "side" : "` + mode + `",
			"executionType" : "LIMIT", "price" : "` + price_str + `",
			"size" : "` + size_str + `"}`)

	ret, err := self.request2Pool("POST", "/private", "/v1/order", "", []byte(order))
	if err != nil {
		return "", err
	}
	var o orderBase
	if err := json.Unmarshal(ret, &o); err != nil {
		return "", err
	}
	if o.Status != 0 {
		return "", fmt.Errorf("%s", o.Message())
	}

	return o.Data, nil
}
