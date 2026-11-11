package recipients

import (
	"errors"
	"strings"
	"testing"
)

func TestDomainNameIsLowercased(t *testing.T) {
	got, err := DomainNames{}.Normalize(" Example.COM ")
	if err != nil || got != "example.com" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDomainNameWithHyphensInsideIsAccepted(t *testing.T) {
	if _, err := (DomainNames{}).Normalize("xn--bcher-kva.example"); err != nil {
		t.Fatal(err)
	}
}

func TestDomainNameWithoutADotIsRefused(t *testing.T) {
	_, err := DomainNames{}.Normalize("mesail-operators")
	var invalid InvalidDomainName
	if !errors.As(err, &invalid) {
		t.Fatalf("got %v", err)
	}
}

func TestDomainNameWithAnUnderscoreIsRefused(t *testing.T) {
	if _, err := (DomainNames{}).Normalize("ex_ample.com"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestDomainNameWithALeadingHyphenIsRefused(t *testing.T) {
	if _, err := (DomainNames{}).Normalize("-example.com"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestDomainNameWithAnEmptyLabelIsRefused(t *testing.T) {
	if _, err := (DomainNames{}).Normalize("example..com"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestDomainNameWithASlashIsRefused(t *testing.T) {
	if _, err := (DomainNames{}).Normalize("example.com/../x"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestDomainNameLongerThan253IsRefused(t *testing.T) {
	label := strings.Repeat("a", 63)
	name := label + "." + label + "." + label + "." + label + ".com"
	if _, err := (DomainNames{}).Normalize(name); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestDomainNameWithALabelLongerThan63IsRefused(t *testing.T) {
	if _, err := (DomainNames{}).Normalize(strings.Repeat("a", 64) + ".com"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestInvalidDomainNameNamesTheName(t *testing.T) {
	if got := (InvalidDomainName{Name: "x"}).Error(); got != `"x" is not a valid domain name` {
		t.Fatalf("got %q", got)
	}
}
