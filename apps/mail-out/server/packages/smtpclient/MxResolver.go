package smtpclient

import (
	"context"
	"net"
)

type MxResolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
}
