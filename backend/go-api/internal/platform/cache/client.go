package cache

type Client struct {
	Addr string
}

func NewClient(addr string) Client {
	return Client{Addr: addr}
}
