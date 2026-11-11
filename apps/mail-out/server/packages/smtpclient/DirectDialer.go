package smtpclient

import (
	"context"
	"net"
	"time"
)

type DirectDialer struct {
	Timeout time.Duration
}

func (dialer DirectDialer) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	return (&net.Dialer{Timeout: dialer.Timeout}).DialContext(ctx, network, address)
}
