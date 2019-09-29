go-gmo-coin
===

* the library for easy use of [gmo coin](https://api.coin.z.com/).

---

## sample

* easy
```
import "github.com/hinoshiba/go-gmo-coin/gomocoin"

func main() {
	API_KEY = "your api key"
	SECRET_KEY = "your secret key"

	gmocoin, err := gomocoin.NewGoMOcoin(API_KEY, SECRET_KEY, context.Background())
	if err != nil {
		panic(err)
	}
	defer gmocoin.Close()

	_, err = gmocoin.UpdateRate()
	if err != nil {
		panic(err)
	}

	//sell
	id, err := gmocoin.Order(gomocoin.SIDE_SELL, gomocoin.SYMBOL_BTC, 0.0001)
	if err != nil {
		panic(err)
	}

	//buy
	id, err = gmocoin.Order(gomocoin.SIDE_BUY, gomocoin.SYMBOL_BTC, 0.0001)
	if err != nil {
		panic(err)
	}
}
```

* manual
```
import "github.com/hinoshiba/go-gmo-coin/gomocoin"
import "log"

func main() {
	API_KEY = "your api key"
	SECRET_KEY = "your secret key"

	auth := gomocoin.NewAuth(API_KEY, SECRET_KEY)
	clnt := gomocoin.NewClient(auth)

//manual request.
	req, err := clnt.NewRequest("GET", "/public", "/v1/status", nil)
	if err != nil {
		panic(err)
	}
	ret, err := req.Do()
	if err != nil {
		panic(err)
	}
	log.Println(string(ret))


//use a pool of have wait timer on 300 miliseconds.
	ctx, _ := context.WithCancel(context.Background())
	clnt.RunPool(&ctx)

	req, err = clnt.NewRequest("GET", "/public", "/v1/status", nil)
	if err != nil {
		panic(err)
	}
	ret, err = clnt.PostPool(req)
	if err != nil {
		panic(err)
	}
	log.Println(string(ret))
}
```
