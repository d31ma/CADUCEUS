package smtpclient

import (
	"context"
	"net"
)

type Dialer interface {
	DialContext(ctx context.Context, network string, address string) (net.Conn, error)
}
