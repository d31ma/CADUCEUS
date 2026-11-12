package dnscheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

type SystemLookup struct {
	resolver systemResolver
	timeout  time.Duration
}

func NewSystemLookup(timeout time.Duration) SystemLookup {
	return SystemLookup{resolver: net.DefaultResolver, timeout: timeout}
}

func (lookup SystemLookup) Mx(name string) ([]string, error) {
	bounded, cancel := context.WithTimeout(context.Background(), lookup.timeout)
	defer cancel()
	records, err := lookup.resolver.LookupMX(bounded, name)
	return lookup.mxHosts(records), lookup.classify(name, err)
}

func (lookup SystemLookup) Txt(name string) ([]string, error) {
	bounded, cancel := context.WithTimeout(context.Background(), lookup.timeout)
	defer cancel()
	records, err := lookup.resolver.LookupTXT(bounded, name)
	return append([]string{}, records...), lookup.classify(name, err)
}

func (lookup SystemLookup) mxHosts(records []*net.MX) []string {
	hosts := []string{}
	for _, record := range records {
		hosts = append(hosts, hostName{}.normalize(record.Host))
	}
	return hosts
}

func (SystemLookup) classify(name string, err error) error {
	var dnsError *net.DNSError
	if err == nil || (errors.As(err, &dnsError) && dnsError.IsNotFound) {
		return nil
	}
	return fmt.Errorf("dnscheck: query %q: %w", name, err)
}
