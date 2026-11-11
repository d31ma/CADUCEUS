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
	if realm.Settings.ServiceTokens != "https://mail.mesail.test/api/v1/identity/service-token" || realm.Name != "" {
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

func TestIssuerPatternResolvesAFixedPatternAsItIs(t *testing.T) {
	resolved, err := IssuerPattern{Pattern: "https://x/realms/mesail"}.Resolved("ignored")
	expanded, expandErr := IssuerPattern{Pattern: "https://x/realms/{realm}"}.Resolved("example.com")
	if err != nil || resolved != "https://x/realms/mesail" || expandErr != nil || expanded != "https://x/realms/example.com" {
		t.Fatalf("got %q %v, %q %v", resolved, err, expanded, expandErr)
	}
}
