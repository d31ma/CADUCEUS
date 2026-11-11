package oidc

import (
	"errors"
	"testing"
)

func TestFromHeaderReturnsTheToken(t *testing.T) {
	token, err := BearerToken{}.FromHeader("Bearer abc.def.ghi")
	if err != nil || token != "abc.def.ghi" {
		t.Fatalf("got %q, %v", token, err)
	}
}

func TestFromHeaderRefusesAnEmptyHeader(t *testing.T) {
	_, err := BearerToken{}.FromHeader("")
	if !errors.Is(err, MissingBearerToken{}) {
		t.Fatalf("got %v", err)
	}
}

func TestFromHeaderRefusesBasicAuth(t *testing.T) {
	if _, err := (BearerToken{}).FromHeader("Basic dXNlcjpwYXNz"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestFromHeaderRefusesABlankToken(t *testing.T) {
	if _, err := (BearerToken{}).FromHeader("Bearer    "); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestMissingBearerTokenExplainsItself(t *testing.T) {
	if (MissingBearerToken{}).Error() == "" {
		t.Fatal("empty message")
	}
}
