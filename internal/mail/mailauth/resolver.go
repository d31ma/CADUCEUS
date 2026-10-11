package mailauth

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sort"
)

// Resolver is the DNS the verifier queries. Errors that mean "no such name"
// or "no records of this type" must satisfy IsNotFound; timeouts IsTimeout.
type Resolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
	// LookupIP answers the A (network "ip4") or AAAA ("ip6") records.
	LookupIP(ctx context.Context, network, host string) ([]netip.Addr, error)
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupAddr(ctx context.Context, address string) ([]string, error)
}

// DNSError is a classified lookup failure, the counterpart of Node's
// ENOTFOUND/ENODATA, ETIMEOUT and EREFUSED codes.
type DNSError struct {
	Name     string
	NotFound bool
	Timeout  bool
	Refused  bool
	Message  string
}

func (err *DNSError) Error() string { return err.Message }

// IsNotFound reports a name or record type that does not exist.
func IsNotFound(err error) bool {
	var dns *DNSError
	if errors.As(err, &dns) {
		return dns.NotFound
	}
	var netErr *net.DNSError
	return errors.As(err, &netErr) && netErr.IsNotFound
}

// IsTimeout reports a lookup that timed out.
func IsTimeout(err error) bool {
	var dns *DNSError
	if errors.As(err, &dns) {
		return dns.Timeout
	}
	var netErr *net.DNSError
	return errors.As(err, &netErr) && netErr.IsTimeout
}

func isRefused(err error) bool {
	var dns *DNSError
	return errors.As(err, &dns) && dns.Refused
}

func code(err error) string {
	switch {
	case IsNotFound(err):
		return "ENOTFOUND"
	case IsTimeout(err):
		return "ETIMEOUT"
	case isRefused(err):
		return "EREFUSED"
	}
	return err.Error()
}

// SystemResolver queries the host's DNS.
type SystemResolver struct{ Resolver *net.Resolver }

func (system SystemResolver) resolver() *net.Resolver {
	if system.Resolver != nil {
		return system.Resolver
	}
	return net.DefaultResolver
}

// LookupTXT answers each TXT record with its strings joined.
func (system SystemResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return system.resolver().LookupTXT(ctx, name)
}

// LookupIP answers A or AAAA records.
func (system SystemResolver) LookupIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return system.resolver().LookupNetIP(ctx, network, host)
}

// LookupMX answers the MX records sorted by preference.
func (system SystemResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	records, err := system.resolver().LookupMX(ctx, name)
	sort.SliceStable(records, func(a, b int) bool { return records[a].Pref < records[b].Pref })
	return records, err
}

// LookupAddr answers PTR names.
func (system SystemResolver) LookupAddr(ctx context.Context, address string) ([]string, error) {
	return system.resolver().LookupAddr(ctx, address)
}
