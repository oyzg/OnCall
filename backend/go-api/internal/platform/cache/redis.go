package cache

import (
	"context"
	"net"
	"time"
)

type Probe struct {
	Addr    string
	Timeout time.Duration
}

func (p Probe) Ping(ctx context.Context) error {
	dialer := net.Dialer{Timeout: p.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", p.Addr)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
