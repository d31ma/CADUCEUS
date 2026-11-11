package addresses

import (
	"errors"
	"testing"
)

func assertRefusedAs(t *testing.T, err error, kind string) {
	t.Helper()
	var invalid InvalidName
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidName, got %v", err)
	}
	if invalid.Kind != kind {
		t.Fatalf("expected kind %q, got %q (%v)", kind, invalid.Kind, err)
	}
}

func assertText(t *testing.T, got string, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func assertAccepted(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestInvalidNameErrorJoinsKindAndReason(t *testing.T) {
	err := InvalidName{Kind: "username", Reason: "must be 1 to 64 characters"}
	assertText(t, err.Error(), "username must be 1 to 64 characters")
}

func assertTruth(t *testing.T, got bool, want bool) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
