package apierrors

import "testing"

func TestFormatPutsTheNumberFirstWithFourDigits(t *testing.T) {
	got := Message{}.Format(510, "This passkey name is already used.")
	if got != "[MS0510]: This passkey name is already used." {
		t.Fatalf("got %q", got)
	}
}

func TestFormatKeepsAFourDigitNumber(t *testing.T) {
	if got := (Message{}).Format(5500, "Something went wrong."); got != "[MS5500]: Something went wrong." {
		t.Fatalf("got %q", got)
	}
}

func TestFormatWritesNumberZero(t *testing.T) {
	if got := (Message{}).Format(0, "Something went wrong."); got != "[MS0000]: Something went wrong." {
		t.Fatalf("got %q", got)
	}
}
