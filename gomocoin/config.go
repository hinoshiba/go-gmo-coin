package gomocoin

type Auth struct {
	key    string
	secret string
}

func NewAuth(api_key string, secret_key string) *Auth {
	return &Auth{key:api_key, secret:secret_key}
}
