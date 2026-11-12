package dnscheck

import (
	"context"
	"net"
)

type systemResolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupTXT(ctx context.Context, name string) ([]string, error)
}
