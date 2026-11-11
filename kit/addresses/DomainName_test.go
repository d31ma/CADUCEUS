package addresses

import (
	"strings"
	"testing"
)

func assertDomain(t *testing.T, input string, want string) {
	t.Helper()
	got, err := DomainName{}.Normalized(input)
	assertAccepted(t, err)
	assertText(t, got, want)
}

func assertDomainRefused(t *testing.T, input string) {
	t.Helper()
	got, err := DomainName{}.Normalized(input)
	assertRefusedAs(t, err, NameKinds{}.Domain())
	assertText(t, got, "")
}

func TestDomainNameTrimsAndLowers(t *testing.T) {
	assertDomain(t, " Example.COM ", "example.com")
}

func TestDomainNameAllowsSingleLabel(t *testing.T) {
	assertDomain(t, "localhost", "localhost")
}

func TestDomainNameAllowsALabel(t *testing.T) {
	assertDomain(t, "xn--bcher-kva.example", "xn--bcher-kva.example")
}

func TestDomainNameAllowsInnerDash(t *testing.T) {
	assertDomain(t, "mail-1.example.org", "mail-1.example.org")
}

func TestDomainNameAllowsSixtyThreeCharacterLabel(t *testing.T) {
	label := strings.Repeat("a", 63)
	assertDomain(t, label+".com", label+".com")
}

func TestDomainNameAllowsTwoHundredFiftyThreeCharacters(t *testing.T) {
	label := strings.Repeat("a", 63)
	name := label + "." + label + "." + label + "." + strings.Repeat("b", 61)
	assertDomain(t, name, name)
}

func TestDomainNameRefusesTwoHundredFiftyFourCharacters(t *testing.T) {
	label := strings.Repeat("a", 63)
	assertDomainRefused(t, label+"."+label+"."+label+"."+strings.Repeat("b", 62))
}

func TestDomainNameRefusesSixtyFourCharacterLabel(t *testing.T) {
	assertDomainRefused(t, strings.Repeat("a", 64)+".com")
}

func TestDomainNameRefusesEmpty(t *testing.T) {
	assertDomainRefused(t, "")
}

func TestDomainNameRefusesStar(t *testing.T) {
	assertDomainRefused(t, "*")
}

func TestDomainNameRefusesWildcardLabel(t *testing.T) {
	assertDomainRefused(t, "*.example.com")
}

func TestDomainNameRefusesLeadingDash(t *testing.T) {
	assertDomainRefused(t, "-example.com")
}

func TestDomainNameRefusesTrailingDash(t *testing.T) {
	assertDomainRefused(t, "example-.com")
}

func TestDomainNameRefusesEmptyLabel(t *testing.T) {
	assertDomainRefused(t, "example..com")
}

func TestDomainNameRefusesTrailingDot(t *testing.T) {
	assertDomainRefused(t, "example.com.")
}

func TestDomainNameRefusesUnderscore(t *testing.T) {
	assertDomainRefused(t, "my_host.com")
}

func TestDomainNameRefusesNonAscii(t *testing.T) {
	assertDomainRefused(t, "bücher.de")
}
