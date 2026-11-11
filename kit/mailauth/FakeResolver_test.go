package mailauth

import (
	"context"
	"net"
	"strings"
)

type FakeResolver struct {
	texts     map[string][]string
	addresses map[string][]net.IP
	exchanges map[string][]*net.MX
	failing   map[string]bool
	asked     []string
}

func newFakeResolver() *FakeResolver {
	return &FakeResolver{texts: map[string][]string{}, addresses: map[string][]net.IP{}, exchanges: map[string][]*net.MX{}, failing: map[string]bool{}}
}

func (fake *FakeResolver) text(name string, records ...string) *FakeResolver {
	fake.texts[name] = append(fake.texts[name], records...)
	return fake
}

func (fake *FakeResolver) address(name string, addresses ...string) *FakeResolver {
	for _, address := range addresses {
		fake.addresses[name] = append(fake.addresses[name], net.ParseIP(address))
	}
	return fake
}

func (fake *FakeResolver) exchange(name string, hosts ...string) *FakeResolver {
	for _, host := range hosts {
		fake.exchanges[name] = append(fake.exchanges[name], &net.MX{Host: host})
	}
	return fake
}

func (fake *FakeResolver) fail(name string) *FakeResolver {
	fake.failing[name] = true
	return fake
}

func (fake *FakeResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return lookupIn(fake, name, fake.texts)
}

func (fake *FakeResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return lookupIn(fake, host, fake.addresses)
}

func (fake *FakeResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	return lookupIn(fake, name, fake.exchanges)
}

func lookupIn[T any](fake *FakeResolver, name string, answers map[string][]T) ([]T, error) {
	key := strings.ToLower(strings.TrimSuffix(name, "."))
	fake.asked = append(fake.asked, key)
	if fake.failing[key] {
		return nil, &net.DNSError{Err: "timeout", Name: key, IsTemporary: true, IsTimeout: true}
	}
	found, exists := answers[key]
	if !exists {
		return nil, &net.DNSError{Err: "no such host", Name: key, IsNotFound: true}
	}
	return found, nil
}
