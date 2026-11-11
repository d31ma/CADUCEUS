package oidc

import (
	"testing"
	"time"
)

func startedRealm(t *testing.T, name string) SigningRealm {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	realm, err := SigningRealm{}.Started(name)
	if err != nil {
		t.Fatal(err)
	}
	document, err := realm.ApplicationJson()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("YON_APPLICATION_JSON", document)
	if err := realm.Publish(time.Now()); err != nil {
		t.Fatal(err)
	}
	return realm
}

func memberRequest() TokenRequest {
	return TokenRequest{Subject: "member-1", Name: "Ana Example", Email: "ana@example.com", RealmRoles: []string{"mesail-member"}, Audience: "mesail-api", AuthorizedParty: "mesail-identity", ExpiresAt: time.Now().Unix() + 60}
}

func TestConfiguredGateAcceptsATokenFromTheSigningRealm(t *testing.T) {
	realm := startedRealm(t, "example.com")
	token, err := realm.Token(memberRequest())
	if err != nil {
		t.Fatal(err)
	}
	caller, err := Configured().Identify("Bearer " + token)
	if err != nil || caller.DomainId != "example.com" || caller.MemberId != "member-1" || caller.Roles[0] != "Member" {
		t.Fatalf("got %+v, %v", caller, err)
	}
}

func TestConfiguredGateRefusesAnExpiredSigningRealmToken(t *testing.T) {
	realm := startedRealm(t, "example.com")
	request := memberRequest()
	request.ExpiresAt = time.Now().Unix() - 60
	token, err := realm.Token(request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Configured().Identify("Bearer " + token)
	refusedBy(t, err)
}

func TestConfiguredGateRefusesASigningRealmTokenForAnotherAudience(t *testing.T) {
	realm := startedRealm(t, "example.com")
	request := memberRequest()
	request.Audience = "account"
	token, err := realm.Token(request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Configured().Identify("Bearer " + token)
	refusedBy(t, err)
}

func TestSigningRealmRefusesAnInvalidRealmName(t *testing.T) {
	realm, err := SigningRealm{}.Started("Not A Domain")
	if err != nil {
		t.Fatal(err)
	}
	_, tokenErr := realm.Token(memberRequest())
	publishErr := realm.Publish(time.Now())
	if tokenErr == nil || publishErr == nil {
		t.Fatalf("got %v, %v", tokenErr, publishErr)
	}
}
