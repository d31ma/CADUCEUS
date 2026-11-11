package compose_test

import (
	"testing"

	"mesail.dev/packages/compose"
)

func TestParseLowerCasesOnlyTheDomain(t *testing.T) {
	address, err := compose.Addresses{}.Parse("Bob Smith <Bob.Smith@Example.ORG>")
	if err != nil {
		t.Fatal(err)
	}
	if address.Name != "Bob Smith" || address.Email != "Bob.Smith@example.org" {
		t.Fatalf("address = %+v", address)
	}
}

func TestParseRefusesBadText(t *testing.T) {
	_, err := compose.Addresses{}.Parse("not an address")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestDomainIsLowerCasePartAfterTheLastAt(t *testing.T) {
	if got := (compose.Addresses{}).Domain("\"a@b\"@Mail.Example.COM"); got != "mail.example.com" {
		t.Fatalf("domain = %q", got)
	}
}
