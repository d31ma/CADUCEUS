package oidc

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

type fakeVerifier struct {
	claims Claims
	err    error
	seen   string
}

func (fake *fakeVerifier) Verify(rawToken string, now time.Time) (Claims, error) {
	fake.seen = rawToken
	return fake.claims, fake.err
}

func fixedNow() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}

func TestIdentifyBuildsTheCallerFromVerifiedClaims(t *testing.T) {
	verifier := &fakeVerifier{claims: Claims{
		Issuer: "https://id.example.com/realms/example.com", Subject: "u-1", Name: "Ana", Email: "ana@example.com",
		RealmRoles: []string{"mesail-member"},
	}}
	caller, err := NewGate(verifier, fixedNow).Identify("Bearer t-1")
	want := Caller{DomainId: "example.com", MemberId: "u-1", DisplayName: "Ana", Email: "ana@example.com", Roles: []string{"Member"}}
	if err != nil || !reflect.DeepEqual(caller, want) || verifier.seen != "t-1" {
		t.Fatalf("got %+v, %v, seen %q", caller, err, verifier.seen)
	}
}

func TestIdentifyRefusesAMissingHeaderWithoutCallingTheVerifier(t *testing.T) {
	verifier := &fakeVerifier{}
	_, err := NewGate(verifier, fixedNow).Identify("")
	if !errors.Is(err, MissingBearerToken{}) || verifier.seen != "" {
		t.Fatalf("got %v, seen %q", err, verifier.seen)
	}
}

func TestIdentifyPassesOnTheVerifierRefusal(t *testing.T) {
	verifier := &fakeVerifier{err: errors.New("expired")}
	if _, err := NewGate(verifier, fixedNow).Identify("Bearer t-1"); err == nil || err.Error() != "expired" {
		t.Fatalf("got %v", err)
	}
}

func TestIdentifyRefusesAnIssuerThatIsNotARealm(t *testing.T) {
	verifier := &fakeVerifier{claims: Claims{Issuer: "https://id.example.com/", Subject: "u-1"}}
	_, err := NewGate(verifier, fixedNow).Identify("Bearer t-1")
	var unknown UnknownIssuer
	if !errors.As(err, &unknown) {
		t.Fatalf("got %v", err)
	}
}

func TestConfiguredGateFailsClosed(t *testing.T) {
	_, err := Configured().Identify("Bearer anything")
	if !errors.Is(err, NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestRealmNameAcceptsATrailingSlash(t *testing.T) {
	name, err := Realm{}.Name("https://id.example.com/realms/acme.org/")
	if err != nil || name != "acme.org" {
		t.Fatalf("got %q, %v", name, err)
	}
}

func TestRealmNameRefusesANestedPath(t *testing.T) {
	if _, err := (Realm{}).Name("https://id.example.com/realms/acme.org/extra"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestErrorsExplainThemselves(t *testing.T) {
	messages := []string{NotConfigured{}.Error(), UnknownIssuer{Issuer: "x"}.Error()}
	for _, message := range messages {
		if message == "" {
			t.Fatal("empty message")
		}
	}
}
