package grpcclient

type Client struct {
	Target string
}

func NewClient(target string) Client {
	return Client{Target: target}
}
