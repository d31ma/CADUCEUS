package dnscheck

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
	"time"
)

type fakeResolver struct {
	exchanges []*net.MX
	texts     []string
	err       error
	names     []string
	deadline  bool
}

func (resolver *fakeResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	resolver.names = append(resolver.names, name)
	_, resolver.deadline = ctx.Deadline()
	return resolver.exchanges, resolver.err
}

func (resolver *fakeResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	resolver.names = append(resolver.names, name)
	_, resolver.deadline = ctx.Deadline()
	return resolver.texts, resolver.err
}

func systemLookup(resolver *fakeResolver) SystemLookup {
	return SystemLookup{resolver: resolver, timeout: time.Second}
}

func TestNewSystemLookupUsesTheHostResolver(t *testing.T) {
	lookup := NewSystemLookup(time.Second)
	if lookup.resolver != net.DefaultResolver || lookup.timeout != time.Second {
		t.Fatalf("got %#v", lookup)
	}
}

func TestSystemMxReturnsHostsWithoutDotInLowerCase(t *testing.T) {
	resolver := &fakeResolver{exchanges: []*net.MX{{Host: "MX.Example.NET.", Pref: 10}, {Host: "backup.example.net.", Pref: 20}}}
	got, err := systemLookup(resolver).Mx("example.com")
	if err != nil || !slices.Equal(got, []string{"mx.example.net", "backup.example.net"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if !slices.Equal(resolver.names, []string{"example.com"}) || !resolver.deadline {
		t.Fatalf("asked %v with deadline %v", resolver.names, resolver.deadline)
	}
}

func TestSystemTxtReturnsTheRecords(t *testing.T) {
	resolver := &fakeResolver{texts: []string{"v=spf1 mx -all", "other"}}
	got, err := systemLookup(resolver).Txt("example.com")
	if err != nil || !slices.Equal(got, []string{"v=spf1 mx -all", "other"}) || !resolver.deadline {
		t.Fatalf("got %v, %v, deadline %v", got, err, resolver.deadline)
	}
}

func TestSystemMxTreatsAMissingNameAsNoRecords(t *testing.T) {
	resolver := &fakeResolver{err: &net.DNSError{Err: "no such host", Name: "example.com", IsNotFound: true}}
	got, err := systemLookup(resolver).Mx("example.com")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSystemTxtTreatsAMissingNameAsNoRecords(t *testing.T) {
	resolver := &fakeResolver{err: &net.DNSError{Err: "no such host", Name: "_dmarc.example.com", IsNotFound: true}}
	got, err := systemLookup(resolver).Txt("_dmarc.example.com")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSystemMxReportsAResolverFailure(t *testing.T) {
	cause := &net.DNSError{Err: "server misbehaving", Name: "example.com", IsTemporary: true}
	_, err := systemLookup(&fakeResolver{err: cause}).Mx("example.com")
	if !errors.Is(err, cause) {
		t.Fatalf("got %v", err)
	}
}

func TestSystemTxtReportsATimeout(t *testing.T) {
	_, err := systemLookup(&fakeResolver{err: context.DeadlineExceeded}).Txt("example.com")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
}

func TestCheckerWorksWithTheSystemLookup(t *testing.T) {
	resolver := &fakeResolver{exchanges: []*net.MX{{Host: "mx.mesail.test.", Pref: 10}}}
	states, err := NewChecker(systemLookup(resolver)).Check("example.com", Expected{MailHost: "mx.mesail.test", SpfMechanism: "mx", DkimSelector: "mesail"})
	if err != nil || states.Mx != "ok" {
		t.Fatalf("got %+v, %v", states, err)
	}
}
