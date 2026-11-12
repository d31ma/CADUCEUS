package oidc

import (
	"testing"
	"time"
)

func TestConfiguredGateRefusesAnExpiredSigningRealmToken(t *testing.T) {
	realm := startedMesailRealm(t)
	request := mesailMemberRequest()
	request.ExpiresAt = time.Now().Unix() - 60
	token, err := realm.Token(request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Configured().Identify("Bearer " + token)
	refusedBy(t, err)
}

func TestConfiguredGateRefusesASigningRealmTokenForAnotherAudience(t *testing.T) {
	realm := startedMesailRealm(t)
	request := mesailMemberRequest()
	request.Audience = "account"
	token, err := realm.Token(request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Configured().Identify("Bearer " + token)
	refusedBy(t, err)
}

func startedMesailRealm(t *testing.T) SigningRealm {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	realm, err := SigningRealm{}.StartedMesail()
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

func mesailMemberRequest() TokenRequest {
	return TokenRequest{Subject: "person-1", Name: "Ana Example", Username: "ana", Email: "ana@example.com", Domain: "example.com", RealmRoles: []string{"mesail-member"}, Audience: "mesail-api", AuthorizedParty: "mesail-web", ExpiresAt: time.Now().Unix() + 60}
}

func TestConfiguredGateAcceptsATokenFromTheMesailSigningRealm(t *testing.T) {
	realm := startedMesailRealm(t)
	token, err := realm.Token(mesailMemberRequest())
	if err != nil {
		t.Fatal(err)
	}
	caller, err := Configured().Identify("Bearer " + token)
	if err != nil || caller.DomainId != "example.com" || caller.MemberId != "person-1" || caller.Username != "ana" {
		t.Fatalf("got %+v, %v", caller, err)
	}
}

func TestMesailSigningRealmSettingsNameTheServiceTokenRoute(t *testing.T) {
	realm := startedMesailRealm(t)
	if realm.Settings.ServiceTokens != "https://mail.mesail.test/api/v1/identity/service-token" {
		t.Fatalf("got %+v", realm.Settings)
	}
}

func TestConfiguredGateRefusesAMesailSigningRealmTokenForAKeycloakClient(t *testing.T) {
	realm := startedMesailRealm(t)
	request := mesailMemberRequest()
	request.AuthorizedParty = "mesail-identity"
	token, err := realm.Token(request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Configured().Identify("Bearer " + token)
	refusedBy(t, err)
}

func TestConfiguredGateRefusesAMesailSigningRealmTokenWithoutADomain(t *testing.T) {
	realm := startedMesailRealm(t)
	request := mesailMemberRequest()
	request.Domain = ""
	token, err := realm.Token(request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Configured().Identify("Bearer " + token)
	refusedBy(t, err)
}

func TestSigningRealmWithABrokenKeyCannotPublish(t *testing.T) {
	realm := startedMesailRealm(t)
	realm.key = brokenCurveKey()
	if err := realm.Publish(time.Now()); err == nil {
		t.Fatal("published a broken key")
	}
}

func TestSigningRealmForADomainNamesItInEveryToken(t *testing.T) {
	realm := startedMesailRealm(t).ForDomain("example.org")
	request := mesailMemberRequest()
	request.Domain = ""
	token, err := realm.Token(request)
	if err != nil {
		t.Fatal(err)
	}
	caller, err := Configured().Identify("Bearer " + token)
	if err != nil || caller.DomainId != "example.org" || realm.Domain != "example.org" {
		t.Fatalf("got %+v, %v", caller, err)
	}
}

func TestSigningRealmKeepsTheDomainOfTheRequest(t *testing.T) {
	realm := startedMesailRealm(t).ForDomain("example.org")
	token, err := realm.Token(mesailMemberRequest())
	if err != nil {
		t.Fatal(err)
	}
	caller, err := Configured().Identify("Bearer " + token)
	if err != nil || caller.DomainId != "example.com" {
		t.Fatalf("got %+v, %v", caller, err)
	}
}
