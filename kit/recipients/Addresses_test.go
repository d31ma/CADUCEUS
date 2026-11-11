package recipients

import (
	"errors"
	"testing"
)

func TestAddressIsLowercasedAndTrimmed(t *testing.T) {
	got, err := Addresses{}.Normalize("  Bo@Example.COM ")
	if err != nil || got != "bo@example.com" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestAddressWithoutAnAtSignIsRefused(t *testing.T) {
	_, err := Addresses{}.Normalize("bo.example.com")
	var invalid InvalidAddress
	if !errors.As(err, &invalid) {
		t.Fatalf("got %v", err)
	}
}

func TestAddressWithTwoAtSignsIsRefused(t *testing.T) {
	if _, err := (Addresses{}).Normalize("bo@x@example.com"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestAddressWithAnEmptyLocalPartIsRefused(t *testing.T) {
	if _, err := (Addresses{}).Normalize("@example.com"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestAddressWithSpacesIsRefused(t *testing.T) {
	if _, err := (Addresses{}).Normalize("b o@example.com"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestAddressWithADomainWithoutADotIsRefused(t *testing.T) {
	if _, err := (Addresses{}).Normalize("bo@localhost"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestAddressWithALongLocalPartIsRefused(t *testing.T) {
	long := "a123456789a123456789a123456789a123456789a123456789a123456789abcde"
	if _, err := (Addresses{}).Normalize(long + "@example.com"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestDomainIsThePartAfterTheAtSign(t *testing.T) {
	if got := (Addresses{}).Domain("bo@example.com"); got != "example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestInvalidAddressNamesTheAddress(t *testing.T) {
	if got := (InvalidAddress{Address: "x"}).Error(); got != `"x" is not a valid email address` {
		t.Fatalf("got %q", got)
	}
}
