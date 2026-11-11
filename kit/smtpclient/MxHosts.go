package smtpclient

import (
	"context"
	"errors"
	"net"
	"slices"
	"strings"
)

type MxHosts struct {
	Resolver MxResolver
}

func (hosts MxHosts) ForDomain(ctx context.Context, domain string) ([]string, error) {
	records, err := hosts.Resolver.LookupMX(ctx, domain)
	records, err = hosts.withImplicit(domain, records, err)
	if err != nil {
		return nil, err
	}
	if err := hosts.refuseNull(domain, records); err != nil {
		return nil, err
	}
	return hosts.names(hosts.sorted(records)), nil
}

func (hosts MxHosts) withImplicit(domain string, records []*net.MX, err error) ([]*net.MX, error) {
	if hosts.missing(records, err) {
		return []*net.MX{{Host: domain}}, nil
	}
	return records, hosts.lookupFailure(domain, err)
}

func (hosts MxHosts) missing(records []*net.MX, err error) bool {
	var dnsError *net.DNSError
	found := errors.As(err, &dnsError)
	return (found && dnsError.IsNotFound) || (err == nil && len(records) == 0)
}

func (hosts MxHosts) lookupFailure(domain string, err error) error {
	if err == nil {
		return nil
	}
	return failures{}.temporary(domain, err)
}

func (hosts MxHosts) refuseNull(domain string, records []*net.MX) error {
	if len(records) != 1 || strings.TrimSuffix(records[0].Host, ".") != "" {
		return nil
	}
	return failures{}.permanent(domain, failures{}.nullMx())
}

func (hosts MxHosts) sorted(records []*net.MX) []*net.MX {
	ordered := slices.Clone(records)
	slices.SortStableFunc(ordered, func(first *net.MX, second *net.MX) int { return int(first.Pref) - int(second.Pref) })
	return ordered
}

func (hosts MxHosts) names(records []*net.MX) []string {
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, strings.TrimSuffix(record.Host, "."))
	}
	return names
}
