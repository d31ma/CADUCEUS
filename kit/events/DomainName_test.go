package events

import (
	"errors"
	"strings"
	"testing"
)

func checkDomainRefused(t *testing.T, domainId string, number int) {
	t.Helper()
	var invalid InvalidEnvelope
	err := DomainName{}.Check(domainId)
	if !errors.As(err, &invalid) || invalid.Number() != number {
		t.Fatalf("domainId %q: expected InvalidEnvelope number %d, got %v", domainId, number, err)
	}
}

func TestDomainNameAcceptsLowercaseDomain(t *testing.T) {
	err := DomainName{}.Check("mail.example-1.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDomainNameRefusesUppercase(t *testing.T) {
	checkDomainRefused(t, "Example.com", 3)
}

func TestDomainNameRefusesNoDot(t *testing.T) {
	checkDomainRefused(t, "localhost", 4)
}

func TestDomainNameRefusesEmpty(t *testing.T) {
	checkDomainRefused(t, "", 4)
}

func TestDomainNameRefusesEmptyLabel(t *testing.T) {
	checkDomainRefused(t, "example..com", 5)
}

func TestDomainNameRefusesLeadingDot(t *testing.T) {
	checkDomainRefused(t, ".example.com", 5)
}

func TestDomainNameRefusesBadCharacters(t *testing.T) {
	checkDomainRefused(t, "exa_mple.com", 3)
}

func TestDomainNameRefusesTooLong(t *testing.T) {
	checkDomainRefused(t, strings.Repeat("a", 250)+".com", 2)
}

func TestDomainNameAcceptsLongestDomain(t *testing.T) {
	err := DomainName{}.Check(strings.Repeat("a", 249) + ".com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
