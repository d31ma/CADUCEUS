package collections

import (
	"regexp"
	"testing"
)

func fyloAcceptsTheName(name string) bool {
	return regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(name)
}

func TestForDomainUsesOnlyCharactersFyloAccepts(t *testing.T) {
	name := CollectionName{}.ForDomain("members", "example.com")
	if !fyloAcceptsTheName(name) || len(name) != len("members-")+16 {
		t.Fatalf("got %q", name)
	}
}

func TestForDomainMonthEndsWithTheMonth(t *testing.T) {
	name := CollectionName{}.ForDomainMonth("messages", "example.com", "202611")
	if !fyloAcceptsTheName(name) || name[len(name)-7:] != "-202611" {
		t.Fatalf("got %q", name)
	}
}

func TestDomainKeyIgnoresCaseAndSpaces(t *testing.T) {
	if (CollectionName{}).DomainKey(" Example.COM ") != (CollectionName{}).DomainKey("example.com") {
		t.Fatal("the key must not depend on case or spaces")
	}
}

func TestDomainsThatLookAlikeGetDifferentKeys(t *testing.T) {
	first := CollectionName{}.DomainKey("a-b.com")
	second := CollectionName{}.DomainKey("a.b-com")
	if first == second {
		t.Fatalf("both domains got %q", first)
	}
}

func TestDomainKeyIsStable(t *testing.T) {
	if got := (CollectionName{}).DomainKey("example.com"); got != "a379a6f6eeafb9a5" {
		t.Fatalf("got %q; the key must never change, or stored data is lost", got)
	}
}
