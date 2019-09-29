package gomocoin

import (
	"io/ioutil"
	"fmt"
	"net/http"
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

	LIMIT_MILLISEC int = 300
)

type Client struct {
	auth  *Auth

	pr_c  chan *poolRequest
}

func NewClient(auth *Auth) *Client {
	pr_c := make(chan *poolRequest)
	return &Client{auth:auth, pr_c:pr_c}
}

func (self *Client) NewRequest(method string, base_path string, path string, body []byte) (*Request, error) {
	if body == nil {
		body = []byte{}
	}

	t_stmp := NewTimestamp()
	sig := self.genhmac([]byte(t_stmp.UnixString() + method + path + string(body)))

	url := URL_BASE + base_path + path

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("API-KEY", self.auth.key)
	req.Header.Set("API-TIMESTAMP", t_stmp.UnixString())
	req.Header.Set("API-SIGN", string(sig))
	req.Header.Add("content-type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	return &Request{c:new(http.Client), r:req}, nil
}

func (self *Client) RunPool(ctx *context.Context) {
	tmr := time.NewTimer(time.Millisecond * time.Duration(LIMIT_MILLISEC))

	go func() {
		for {
			select {
			case <- (*ctx).Done():
				return
			case pr := <- self.pr_c:
				select {
				case <- (*ctx).Done():
					return
				case <- tmr.C:
					b, err := pr.req.Do()
					if err != nil {
						continue
					}
					go func() {
						pr.ret = b
						pr.done <- struct{}{}
					}()
				}
			}
		}
	}()
}

func (self *Client) PostPool(r *Request) ([]byte, error) {
	if self.pr_c == nil {
		return nil, fmt.Errorf("undefined pool channel.")
	}

	pr := newPoolRequest(r)
	go func() {
		self.pr_c <- pr
	}()
	<-pr.done
	return pr.Bytes(), nil
}

type poolRequest struct {
	req  *Request

	done chan struct{}
	ret  []byte
}

func newPoolRequest(req *Request) *poolRequest {
	done := make(chan struct{})
	return &poolRequest{req:req, done:done}
}

func (self *poolRequest) Bytes() []byte {
	return self.ret
}

type Request struct {
	c  *http.Client
	r  *http.Request
}

func (self *Request) Do() ([]byte, error) {
	res, err := self.c.Do(self.r)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
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
