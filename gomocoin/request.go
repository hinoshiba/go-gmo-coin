package gomocoin

import (
	"io/ioutil"
	"fmt"
	"net/http"
	"sync"
	"time"
	"bytes"
	"crypto/sha256"
	"crypto/hmac"
	"strconv"
	"context"
	"encoding/hex"
)

const (
	URL_BASE     = "https://api.coin.z.com"
	PATH_PLIVATE = "/private"
	PATH_PUBLIC  = "/public"

	LIMIT_MILLISEC int = 301
)

type Client struct {
	auth  *Auth

	rq_c  chan *request
}

func NewClient(auth *Auth) *Client {
	rq_c := make(chan *request)
	return &Client{auth:auth, rq_c:rq_c}
}

func (self *Client) NewRequest(method string, base_path string, path string, param string, body []byte) (*Request, error) {
	if body == nil {
		body = []byte{}
	}

	t_stmp := NewTimestamp()
	sig := self.genhmac([]byte(t_stmp.UnixString() + method + path + string(body)))

	var url string
	if param == "" {
		url = URL_BASE + base_path + path
	} else {
		url = URL_BASE + base_path + path + "?" + param
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("API-KEY", self.auth.key)
	req.Header.Set("API-TIMESTAMP", t_stmp.UnixString())
	req.Header.Set("API-SIGN", string(sig))
	req.Header.Add("content-type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	return NewRequest(req), nil
}

func (self *Client) RunPool(ctx context.Context) {
	tmr := time.NewTicker(time.Millisecond * time.Duration(LIMIT_MILLISEC))

	go func() {
		for {
			select {
			case <- ctx.Done():
				return
			case rq := <- self.rq_c:
				select {
				case <- ctx.Done():
					return
				case <- rq.life.C:
					go func() {
						rq.done()
					}()
					continue
				case <- tmr.C:
					b, err := rq.req.Do()
					if err != nil { //TODO: here?
						continue
					}
					go func(b []byte) {
						rq.ret = b
						rq.done()
					}(b)
				}
			}
		}
	}()
}

func (self *Client) PostPool(r *Request) ([]byte, error) {
	if self.rq_c == nil {
		return nil, fmt.Errorf("undefined pool channel.")
	}

	rq := newRequest(r)
	go func() {
		self.rq_c <- rq
	}()

	rq.wait_done()

	if rq.Bytes() == nil {
		return nil, fmt.Errorf("empty return")
	}
	return rq.Bytes(), nil
}

type request struct {
	req   *Request

	life  *time.Timer
	block chan struct{}
	ret   []byte
}

func newRequest(req *Request) *request {
	block := make(chan struct{})
	life := time.NewTimer(time.Second * 3)
	return &request{req:req, block:block, life:life}
}

func (self *request) Bytes() []byte {
	return self.ret
}

func (self *request) wait_done() {
	<-self.block
}

func (self *request) done() {
	close(self.block)
}

type Request struct {
	r  *http.Request

	mtx *sync.Mutex
}

func NewRequest(r *http.Request) *Request {
	return &Request {
		r: r,
		mtx: new(sync.Mutex),
	}
}

func (self *Request) Do() ([]byte, error) {
	self.mtx.Lock()
	defer self.mtx.Unlock()

	c := self.createClient()
	res, err := c.Do(self.r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (self *Request) createClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport)
	t.MaxConnsPerHost = 1

	return &http.Client{
		Timeout: time.Second * 10,
		Transport: t,
	}
}

func (self *Client) genhmac(msg []byte) []byte {
	m := hmac.New(sha256.New, []byte(self.auth.secret))
	m.Write(msg)

	msum := m.Sum(nil)
	ret := make([]byte, hex.EncodedLen(len(msum)))
	hex.Encode(ret, msum)

	return ret
}

type Timestamp struct {
	t  time.Time
}

func NewTimestamp() *Timestamp {
	return &Timestamp{t:time.Now()}
}

func (self *Timestamp) UnixString() string {
	return strconv.FormatInt(self.t.UnixNano()/int64(time.Millisecond), 10)
}
