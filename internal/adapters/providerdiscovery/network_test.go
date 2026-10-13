package providerdiscovery

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestSafeFetcherRejectsNonHTTPSAndNonPublicDestinations(t *testing.T) {
	resolver := fakeResolver{addresses: map[string][]net.IPAddr{
		"autoconfig.example.com": {{IP: net.ParseIP("127.0.0.1")}},
	}}
	fetcher := NewSafeFetcher(resolver)
	for _, target := range []string{
		"http://autoconfig.example.com/mail/config-v1.1.xml",
		"https://127.0.0.1/mail/config-v1.1.xml",
		"https://autoconfig.example.com/mail/config-v1.1.xml",
	} {
		if _, err := fetcher.Fetch(context.Background(), target); !errors.Is(err, ErrUnsafeDestination) {
			t.Fatalf("Fetch(%q) error = %v, want ErrUnsafeDestination", target, err)
		}
	}
}

func TestSafeFetcherReturnsOneBoundedSuccessfulDocument(t *testing.T) {
	resolver := fakeResolver{addresses: map[string][]net.IPAddr{
		"autoconfig.example.com": {{IP: net.ParseIP("203.0.113.10")}},
	}}
	fetcher := NewSafeFetcher(resolver)
	fetcher.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("<clientConfig/>")),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}
	body, err := fetcher.Fetch(context.Background(), "https://autoconfig.example.com/mail/config-v1.1.xml")
	if err != nil || string(body) != "<clientConfig/>" {
		t.Fatalf("Fetch() = %q, %v", body, err)
	}

	fetcher.maxBytes = 4
	if _, err := fetcher.Fetch(context.Background(), "https://autoconfig.example.com/mail/config-v1.1.xml"); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("oversized Fetch() error = %v, want ErrResponseTooLarge", err)
	}
}

func TestResolverNormalizesMXHosts(t *testing.T) {
	resolver := Resolver{lookupMX: func(context.Context, string) ([]*net.MX, error) {
		return []*net.MX{{Host: "MX2.EXAMPLE.COM.", Pref: 20}, {Host: "mx1.example.com.", Pref: 10}}, nil
	}}
	hosts, err := resolver.LookupMX(context.Background(), "example.com")
	if err != nil || strings.Join(hosts, ",") != "mx1.example.com,mx2.example.com" {
		t.Fatalf("LookupMX() = %#v, %v", hosts, err)
	}
}

type fakeResolver struct {
	addresses map[string][]net.IPAddr
}

func (resolver fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	return resolver.addresses[host], nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
