package events

import (
	"errors"
	"strings"
	"testing"
)

func checkDomainRefused(t *testing.T, domainId string) {
	t.Helper()
	var invalid InvalidEnvelope
	err := DomainName{}.Check(domainId)
	if !errors.As(err, &invalid) {
		t.Fatalf("domainId %q: expected InvalidEnvelope, got %v", domainId, err)
	}
}

func TestDomainNameAcceptsLowercaseDomain(t *testing.T) {
	err := DomainName{}.Check("mail.example-1.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDomainNameRefusesUppercase(t *testing.T) {
	checkDomainRefused(t, "Example.com")
}

func TestDomainNameRefusesNoDot(t *testing.T) {
	checkDomainRefused(t, "localhost")
}

func TestDomainNameRefusesEmpty(t *testing.T) {
	checkDomainRefused(t, "")
}

func TestDomainNameRefusesEmptyLabel(t *testing.T) {
	checkDomainRefused(t, "example..com")
}

func TestDomainNameRefusesLeadingDot(t *testing.T) {
	checkDomainRefused(t, ".example.com")
}

func TestDomainNameRefusesBadCharacters(t *testing.T) {
	checkDomainRefused(t, "exa_mple.com")
}

func TestDomainNameRefusesTooLong(t *testing.T) {
	checkDomainRefused(t, strings.Repeat("a", 250)+".com")
}

func TestDomainNameAcceptsLongestDomain(t *testing.T) {
	err := DomainName{}.Check(strings.Repeat("a", 249) + ".com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
