package compose_test

import (
	"regexp"
	"testing"

	"mesail.dev/packages/compose"
)

func TestNewMessageIdIsRandomHexAtDomain(t *testing.T) {
	first, err := compose.MessageIds{}.New("example.com")
	if err != nil {
		t.Fatal(err)
	}
	second, err := compose.MessageIds{}.New("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}@example\.com$`).MatchString(first) {
		t.Fatalf("id = %q", first)
	}
	if first == second {
		t.Fatal("two ids are equal")
	}
}

func TestNewMessageIdNeedsADomain(t *testing.T) {
	_, err := compose.MessageIds{}.New("")
	assertMissing(t, err, "domain")
}
