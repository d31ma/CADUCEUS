package mailauth

import (
	"context"
	"errors"
	"net"
)

type dnsLookup struct {
	resolver Resolver
}

func (lookup dnsLookup) text(ctx context.Context, name string) ([]string, error) {
	records, err := lookup.resolver.LookupTXT(ctx, name)
	return records, lookup.classify(err)
}

func (lookup dnsLookup) addresses(ctx context.Context, host string) ([]net.IP, error) {
	addresses, err := lookup.resolver.LookupIP(ctx, host)
	return addresses, lookup.classify(err)
}

func (lookup dnsLookup) exchanges(ctx context.Context, name string) ([]*net.MX, error) {
	exchanges, err := lookup.resolver.LookupMX(ctx, name)
	return exchanges, lookup.classify(err)
}

func (dnsLookup) classify(err error) error {
	var dnsError *net.DNSError
	if err == nil || (errors.As(err, &dnsError) && dnsError.IsNotFound) {
		return nil
	}
	return failure{result: Outcomes{}.TempError()}
}
