package blobs

import (
	"errors"
	"strings"
	"testing"
)

func TestKeyPathStartsWithTheDomain(t *testing.T) {
	if got := (Key{DomainId: "example.com", Name: "a1"}).Path(); got != "example.com/a1" {
		t.Fatalf("got %q", got)
	}
}

func TestCheckAcceptsADomainAndAName(t *testing.T) {
	if err := (KeyRules{}).Check(Key{DomainId: "mail.example-1.com", Name: "att_01HZX-y"}); err != nil {
		t.Fatal(err)
	}
}

func TestCheckAcceptsTheLongestDomain(t *testing.T) {
	label := strings.Repeat("a", 63)
	domain := label + "." + label + "." + label + "." + strings.Repeat("b", 61)
	if err := (KeyRules{}).Check(Key{DomainId: domain, Name: "x"}); err != nil || len(domain) != 253 {
		t.Fatalf("len %d: %v", len(domain), err)
	}
}

func TestCheckRefusesADomainLongerThan253(t *testing.T) {
	label := strings.Repeat("a", 63)
	domain := label + "." + label + "." + label + "." + strings.Repeat("b", 62)
	var bad BadKey
	if err := (KeyRules{}).Check(Key{DomainId: domain, Name: "x"}); !errors.As(err, &bad) || bad.Part != "domain" {
		t.Fatalf("got %v", err)
	}
}

func TestCheckRefusesBadDomains(t *testing.T) {
	for _, domain := range []string{"", "Example.com", "..", "a..b", ".a", "a/b", "a_b.com", "-a.com"} {
		var bad BadKey
		if err := (KeyRules{}).Check(Key{DomainId: domain, Name: "x"}); !errors.As(err, &bad) || bad.Part != "domain" {
			t.Fatalf("%q: got %v", domain, err)
		}
	}
}

func TestCheckRefusesBadNames(t *testing.T) {
	for _, name := range []string{"", "..", "../x", "a/b", "a.b", strings.Repeat("n", 129)} {
		var bad BadKey
		if err := (KeyRules{}).Check(Key{DomainId: "example.com", Name: name}); !errors.As(err, &bad) || bad.Part != "name" {
			t.Fatalf("%q: got %v", name, err)
		}
	}
}

func TestErrorsExplainThemselves(t *testing.T) {
	messages := []string{
		BadKey{Part: "name", Value: "../x"}.Error(), NotFound{Path: "example.com/x"}.Error(), TooLarge{Limit: 5}.Error(),
		SizeMismatch{Expected: 5, Received: 6}.Error(), BadSignature{}.Error(), Expired{}.Error(), S3Failure{Method: "GET", Status: 500}.Error(),
	}
	wanted := []string{`name "../x"`, "example.com/x", "5 bytes", "expected 5 bytes but received 6", "signature", "expired", "GET request answered HTTP 500"}
	for index, message := range messages {
		if !strings.Contains(message, wanted[index]) {
			t.Fatalf("%q does not contain %q", message, wanted[index])
		}
	}
}
