package oidc

import (
	"strings"
	"testing"
	"time"
)

func rfcSecret() string {
	return Totp{}.encoding().EncodeToString([]byte("12345678901234567890"))
}

func TestTotpMatchesTheRfc6238Vectors(t *testing.T) {
	cases := map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"}
	for seconds, want := range cases {
		got, err := Totp{}.Code(rfcSecret(), time.Unix(seconds, 0))
		if err != nil || got != want {
			t.Fatalf("at %d got %q, %v, want %q", seconds, got, err, want)
		}
	}
}

func TestTotpAcceptsOneStepOfDrift(t *testing.T) {
	at := time.Unix(1111111109, 0)
	previous, _ := Totp{}.Code(rfcSecret(), at.Add(-30*time.Second))
	next, _ := Totp{}.Code(rfcSecret(), at.Add(30*time.Second))
	if !(Totp{}).Matches(rfcSecret(), previous, at) || !(Totp{}).Matches(rfcSecret(), next, at) {
		t.Fatal("one step of drift should match")
	}
}

func TestTotpRefusesTwoStepsOfDrift(t *testing.T) {
	at := time.Unix(1111111109, 0)
	old, _ := Totp{}.Code(rfcSecret(), at.Add(-61*time.Second))
	if (Totp{}).Matches(rfcSecret(), old, at) {
		t.Fatal("two steps of drift should not match")
	}
}

func TestTotpRefusesAWrongCode(t *testing.T) {
	if (Totp{}).Matches(rfcSecret(), "000000", time.Unix(59, 0)) {
		t.Fatal("wrong code matched")
	}
}

func TestTotpRefusesABadSecret(t *testing.T) {
	_, err := Totp{}.Code("1!", time.Unix(59, 0))
	if err == nil || (Totp{}).Matches("1!", "287082", time.Unix(59, 0)) {
		t.Fatal("a bad secret should fail")
	}
}

func TestTotpMakesA160BitSecret(t *testing.T) {
	secret, err := Totp{}.NewSecret()
	raw, decodeErr := Totp{}.encoding().DecodeString(secret)
	other, _ := Totp{}.NewSecret()
	if err != nil || decodeErr != nil || len(raw) != 20 || secret == other {
		t.Fatalf("got %q, %v, %v", secret, err, decodeErr)
	}
}

func TestTotpUriNamesTheAccountAndSettings(t *testing.T) {
	uri := Totp{}.Uri("ana@example.com", "JBSWY3DP")
	for _, part := range []string{"otpauth://totp/Mesail:ana@example.com?", "secret=JBSWY3DP", "issuer=Mesail", "digits=6", "period=30", "algorithm=SHA1"} {
		if !strings.Contains(uri, part) {
			t.Fatalf("%q lacks %q", uri, part)
		}
	}
}
