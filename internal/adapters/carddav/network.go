package carddav

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type IPResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type safeRoundTripper struct {
	resolver        IPResolver
	privateNetworks []*net.IPNet
	timeout         time.Duration
}

type insecureTLSContextKey struct{}

func withInsecureTLS(ctx context.Context, allow bool) context.Context {
	return context.WithValue(ctx, insecureTLSContextKey{}, allow)
}

// WithInsecureTLS carries an explicit per-account certificate override to the
// shared DAV transport. It is intentionally not a process-global setting.
func WithInsecureTLS(ctx context.Context, allow bool) context.Context {
	return withInsecureTLS(ctx, allow)
}

func newSafeHTTPClient(resolver IPResolver, privateCIDRs []string, timeout time.Duration) (*http.Client, error) {
	return NewSafeHTTPClient(resolver, privateCIDRs, timeout)
}

// NewSafeHTTPClient builds the DNS-pinned, redirect-bounded DAV transport used
// by both CardDAV and CalDAV adapters.
func NewSafeHTTPClient(resolver IPResolver, privateCIDRs []string, timeout time.Duration) (*http.Client, error) {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	networks, err := parsePrivateNetworks(privateCIDRs)
	if err != nil {
		return nil, err
	}
	roundTripper := &safeRoundTripper{resolver: resolver, privateNetworks: networks, timeout: timeout}
	return &http.Client{Transport: roundTripper, Timeout: timeout, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 || request.URL == nil || request.URL.Scheme != "https" || request.URL.User != nil || request.URL.Hostname() == "" {
			return ErrUnsafeEndpoint
		}
		// Never forward Basic credentials across origins. net/http normally
		// strips them too; doing it explicitly keeps the boundary obvious.
		if len(via) > 0 && !sameOrigin(via[0].URL, request.URL) {
			request.Header.Del("Authorization")
			return ErrUnsafeEndpoint
		}
		return nil
	}}, nil
}

func (transport *safeRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil || request.URL.Scheme != "https" || request.URL.User != nil || request.URL.Hostname() == "" || request.URL.Fragment != "" {
		return nil, ErrUnsafeEndpoint
	}
	host := strings.ToLower(strings.TrimSuffix(request.URL.Hostname(), "."))
	addresses, err := transport.resolve(request.Context(), host)
	if err != nil {
		return nil, err
	}
	port := request.URL.Port()
	if port == "" {
		port = "443"
	}
	if parsed, err := strconv.ParseUint(port, 10, 16); err != nil || parsed == 0 {
		return nil, ErrUnsafeEndpoint
	}
	dialer := &net.Dialer{Timeout: transport.timeout, KeepAlive: 30 * time.Second}
	base := &http.Transport{
		Proxy:                 nil,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host, InsecureSkipVerify: request.Context().Value(insecureTLSContextKey{}) == true}, // #nosec G402 -- explicit per-account operator override
		TLSHandshakeTimeout:   transport.timeout,
		ResponseHeaderTimeout: transport.timeout,
		IdleConnTimeout:       30 * time.Second,
		DisableCompression:    true,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var last error
			for _, address := range addresses {
				connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(address.String(), port))
				if dialErr == nil {
					return connection, nil
				}
				last = dialErr
			}
			if last == nil {
				last = ErrUnsafeEndpoint
			}
			return nil, last
		},
	}
	return base.RoundTrip(request)
}

func (transport *safeRoundTripper) resolve(ctx context.Context, host string) ([]net.IP, error) {
	var resolved []net.IPAddr
	if literal := net.ParseIP(host); literal != nil {
		resolved = []net.IPAddr{{IP: literal}}
	} else {
		var err error
		resolved, err = transport.resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, ErrUnsafeEndpoint
		}
	}
	if len(resolved) == 0 {
		return nil, ErrUnsafeEndpoint
	}
	result := make([]net.IP, 0, len(resolved))
	seen := map[string]bool{}
	for _, address := range resolved {
		if !transport.approved(address.IP) {
			return nil, ErrUnsafeEndpoint
		}
		canonical := address.IP.String()
		if !seen[canonical] {
			seen[canonical] = true
			result = append(result, address.IP)
		}
	}
	return result, nil
}

func (transport *safeRoundTripper) approved(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	if ip.IsPrivate() || ip.IsLoopback() {
		for _, network := range transport.privateNetworks {
			if network.Contains(ip) {
				return true
			}
		}
		return false
	}
	if value := ip.To4(); value != nil && value[0] == 100 && value[1]&0xc0 == 64 {
		return false
	}
	return true
}

func parsePrivateNetworks(values []string) ([]*net.IPNet, error) {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		ip, network, err := net.ParseCIDR(strings.TrimSpace(value))
		if err != nil || ip.String() != network.IP.String() {
			return nil, errors.New("CardDAV private network policy must contain canonical CIDRs")
		}
		ones, bits := network.Mask.Size()
		if !(network.IP.IsPrivate() || network.IP.IsLoopback()) || bits == 32 && ones < 8 || bits == 128 && ones < 7 {
			return nil, errors.New("CardDAV private network policy is too broad")
		}
		result = append(result, network)
	}
	return result, nil
}

func sameOrigin(left, right *url.URL) bool {
	return left != nil && right != nil && strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}
