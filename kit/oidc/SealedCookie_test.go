package oidc

import (
	"bytes"
	"testing"
)

func cookieKey() []byte {
	return bytes.Repeat([]byte{7}, 32)
}

func TestSealedCookieRoundTrips(t *testing.T) {
	sealed, err := SealedCookie{Key: cookieKey()}.Seal("refresh-token", "mesail-refresh-example.com")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := SealedCookie{Key: cookieKey()}.Open(sealed, "mesail-refresh-example.com")
	if err != nil || plain != "refresh-token" {
		t.Fatalf("got %q, %v", plain, err)
	}
}

func TestSealedCookieRefusesAnotherCookieName(t *testing.T) {
	sealed, _ := SealedCookie{Key: cookieKey()}.Seal("refresh-token", "mesail-refresh-example.com")
	if _, err := (SealedCookie{Key: cookieKey()}).Open(sealed, "mesail-refresh-example.org"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestSealedCookieRefusesAnotherKey(t *testing.T) {
	sealed, _ := SealedCookie{Key: cookieKey()}.Seal("refresh-token", "n")
	if _, err := (SealedCookie{Key: bytes.Repeat([]byte{8}, 32)}).Open(sealed, "n"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestSealedCookieRefusesAShortValue(t *testing.T) {
	if _, err := (SealedCookie{Key: cookieKey()}).Open("AAAA", "n"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestSealedCookieRefusesAValueThatIsNotBase64(t *testing.T) {
	if _, err := (SealedCookie{Key: cookieKey()}).Open("***", "n"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestSealedCookieRefusesABadKey(t *testing.T) {
	_, sealErr := SealedCookie{Key: []byte("short")}.Seal("v", "n")
	_, openErr := SealedCookie{Key: []byte("short")}.Open("AAAA", "n")
	if sealErr == nil || openErr == nil {
		t.Fatalf("got %v, %v", sealErr, openErr)
	}
}

func TestSealedCookiesDifferEachTime(t *testing.T) {
	first, _ := SealedCookie{Key: cookieKey()}.Seal("v", "n")
	second, _ := SealedCookie{Key: cookieKey()}.Seal("v", "n")
	if first == second {
		t.Fatal("the nonce was reused")
	}
}
