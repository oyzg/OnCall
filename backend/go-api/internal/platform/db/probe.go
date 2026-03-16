package db

import (
	"context"
	"net"
	"strings"
	"time"
)

type Probe struct {
	DSN     string
	Timeout time.Duration
}

func (p Probe) Ping(ctx context.Context) error {
	address := mysqlAddressFromDSN(p.DSN)
	dialer := net.Dialer{Timeout: p.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

func mysqlAddressFromDSN(dsn string) string {
	start := strings.Index(dsn, "@tcp(")
	if start == -1 {
		return "127.0.0.1:3306"
	}
	start += len("@tcp(")
	end := strings.Index(dsn[start:], ")")
	if end == -1 {
		return "127.0.0.1:3306"
	}
	return dsn[start : start+end]
}
