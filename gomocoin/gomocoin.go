package gomocoin

import (
	"fmt"
	"sync"
	"context"
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

	TYPE_MARKET string = "MARKET"
	TYPE_LIMIT string = "LIMIT"
)

type GoMOcoin struct {
	client      *Client

	rate        map[string]*RateData

	ctx_cancel  *context.CancelFunc
	mtx         *sync.Mutex
}

func NewGoMOcoin(api_key string, secret_key string, b_ctx context.Context) (*GoMOcoin, error) {
	ctx, ctx_cancel := context.WithCancel(b_ctx)
	auth := NewAuth(api_key, secret_key)
	clnt := NewClient(auth)
	clnt.RunPool(&ctx)

	rate := make(map[string]*RateData)
	self := &GoMOcoin{client:clnt, ctx_cancel:&ctx_cancel, rate:rate, mtx:new(sync.Mutex)}
	ok, err := self.checkStatus()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("store not open.")
	}
	return self, nil
}

func (self *GoMOcoin) lock() {
	self.mtx.Lock()
}

func (self *GoMOcoin) unlock() {
	self.mtx.Unlock()
}

func (self *GoMOcoin) Close() {
	(*self.ctx_cancel)()
}

func (self *GoMOcoin) request2Pool(method string, base_path string, path string, body []byte) ([]byte, error) {
	req, err := self.client.NewRequest(method, base_path, path, body)
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
	ret, err := self.request2Pool("GET", "/public", "/v1/status", nil)
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

func (self *GoMOcoin) UpdateRate() (map[string]*RateData, error) {
	self.lock()
	defer self.unlock()

	ret, err := self.request2Pool("GET", "/public", "/v1/ticker", nil)
	if err != nil {
		return nil, err
	}
	var rb rateBase
	if err := json.Unmarshal(ret, &rb); err != nil {
		return nil, err
	}

	data := make(map[string]*RateData)
	for _, rd := range rb.Data {
		data[rd.Symbol] = rd
		self.rate[rd.Symbol] = rd
	}
	self.rate = data

	return data, nil
}

type orderBase struct {
	apibase
	Data string `json:"data"`
}

func (self *GoMOcoin) Order(mode string, symbol string, size float64) (string, error) {
	var price string
	rate, ok := self.rate[symbol]
	if !ok {
		return "", fmt.Errorf("undefined symbol. %s", symbol)
	}
	if mode == SIDE_SELL {
		price = rate.Bid
	}
	if mode == SIDE_BUY {
		price = rate.Ask
	}
	if price == "" {
		return "", fmt.Errorf("undefined mode. %s", mode)
	}
	size_str := fmt.Sprintf("%.4f", size)

	order := (`{"symbol" : "` + symbol + `", "side" : "` + mode + `",
			"executionType" : "LIMIT", "price" : "` + price + `",
			"size" : "` + size_str + `"}`)

	ret, err := self.request2Pool("POST", "/private", "/v1/order", []byte(order))
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
