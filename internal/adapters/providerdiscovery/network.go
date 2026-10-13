// Package providerdiscovery implements the network-facing ports used by the
// account discovery domain. It pins validated public DNS results to prevent
// autoconfig from becoming an SSRF primitive.
package providerdiscovery

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var (
	ErrUnsafeDestination = errors.New("unsafe autoconfig destination")
	ErrResponseTooLarge  = errors.New("autoconfig response exceeds limit")
)

const defaultMaxBytes = int64(1 << 20)

type IPResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type SafeFetcher struct {
	resolver IPResolver
	client   *http.Client
	maxBytes int64
}

func NewSafeFetcher(resolver IPResolver) *SafeFetcher {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &SafeFetcher{resolver: resolver, maxBytes: defaultMaxBytes}
}

func (fetcher *SafeFetcher) Fetch(ctx context.Context, target string) ([]byte, error) {
	parsed, addresses, err := fetcher.resolve(ctx, target)
	if err != nil {
		return nil, err
	}
	client := fetcher.client
	if client == nil {
		client = pinnedClient(parsed.Hostname(), parsed.Port(), addresses)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, ErrUnsafeDestination
	}
	request.Header.Set("Accept", "application/xml,text/xml;q=0.9")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("autoconfig HTTP status %d", response.StatusCode)
	}
	limit := fetcher.maxBytes
	if limit <= 0 {
		limit = defaultMaxBytes
	}
	if response.ContentLength > limit {
		return nil, ErrResponseTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, ErrResponseTooLarge
	}
	return body, nil
}

func (fetcher *SafeFetcher) resolve(ctx context.Context, target string) (*url.URL, []net.IPAddr, error) {
	if len(target) > 2048 {
		return nil, nil, ErrUnsafeDestination
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Hostname() == "" || parsed.Fragment != "" {
		return nil, nil, ErrUnsafeDestination
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return nil, nil, ErrUnsafeDestination
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if net.ParseIP(host) != nil || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return nil, nil, ErrUnsafeDestination
	}
	addresses, err := fetcher.resolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, nil, fmt.Errorf("resolve autoconfig host: %w", err)
	}
	for _, address := range addresses {
		if !publicIP(address.IP) {
			return nil, nil, ErrUnsafeDestination
		}
	}
	return parsed, addresses, nil
}

func publicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	if value := ip.To4(); value != nil && value[0] == 100 && value[1]&0xc0 == 64 {
		return false
	}
	return true
}

func pinnedClient(serverName, requestedPort string, addresses []net.IPAddr) *http.Client {
	port := requestedPort
	if port == "" {
		port = "443"
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var last error
			for _, address := range addresses {
				connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
				if err == nil {
					return connection, nil
				}
				last = err
			}
			return nil, last
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("autoconfig redirects are disabled")
		},
	}
}

type Resolver struct {
	lookupMX func(context.Context, string) ([]*net.MX, error)
}

func (resolver Resolver) LookupMX(ctx context.Context, domain string) ([]string, error) {
	lookup := resolver.lookupMX
	if lookup == nil {
		lookup = net.DefaultResolver.LookupMX
	}
	records, err := lookup(ctx, domain)
	if err != nil {
		return nil, err
	}
	sort.Slice(records, func(left, right int) bool {
		if records[left].Pref == records[right].Pref {
			return records[left].Host < records[right].Host
		}
		return records[left].Pref < records[right].Pref
	})
	hosts := make([]string, 0, len(records))
	for _, record := range records {
		hosts = append(hosts, strings.ToLower(strings.TrimSuffix(record.Host, ".")))
	}
	return hosts, nil
}
