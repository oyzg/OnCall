package grpcclient

import (
	"context"
	"fmt"
	"strings"

	aipb "github.com/oyzg/OnCall/backend/go-api/gen/proto/ai"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	Target  string
	conn    *grpc.ClientConn
	runtime aipb.RuntimeServiceClient
	err     error
}

func NewClient(target string) Client {
	normalizedTarget := strings.TrimSpace(target)
	if normalizedTarget == "" {
		normalizedTarget = "127.0.0.1:50051"
	}

	conn, err := grpc.DialContext(
		context.Background(),
		normalizedTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return Client{
			Target: normalizedTarget,
			err:    err,
		}
	}

	return Client{
		Target:  normalizedTarget,
		conn:    conn,
		runtime: aipb.NewRuntimeServiceClient(conn),
	}
}

func (c Client) RuntimeClient() (aipb.RuntimeServiceClient, error) {
	if c.err != nil {
		return nil, c.err
	}
	if c.runtime == nil {
		return nil, fmt.Errorf("grpc runtime client unavailable for %s", c.Target)
	}
	return c.runtime, nil
}

func (c Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
