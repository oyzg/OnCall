package grpcclient

import (
	"context"
	"net"
	"time"
)

type Probe struct {
	Target  string
	Timeout time.Duration
}

func (p Probe) Ping(ctx context.Context) error {
	dialer := net.Dialer{Timeout: p.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", p.Target)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
