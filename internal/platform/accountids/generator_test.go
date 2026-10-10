package accountids

import (
	"errors"
	"strings"
	"testing"
)

func TestGeneratorCreatesOpaqueContractCompatibleIDs(t *testing.T) {
	generator := New()
	first, err := generator.NewAccountID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := generator.NewAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(string(first), "ext_") || len(first) < 24 {
		t.Fatalf("generated IDs = %q, %q", first, second)
	}
}

func TestGeneratorPropagatesEntropyFailure(t *testing.T) {
	generator := New()
	generator.random = failingReader{}
	if _, err := generator.NewAccountID(); err == nil {
		t.Fatal("NewAccountID() error = nil")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
