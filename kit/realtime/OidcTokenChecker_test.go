package realtime

import (
	"errors"
	"testing"
	"time"

	"mesail.dev/packages/oidc"
)

func tokenRealm(t *testing.T) (oidc.SigningRealm, OidcTokenChecker) {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	realm, err := oidc.SigningRealm{}.Started("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := realm.Publish(time.Now()); err != nil {
		t.Fatal(err)
	}
	return realm, OidcTokenChecker{Verifier: oidc.VerifierFactory{}.Jwks(realm.Settings)}
}

func signedToken(t *testing.T, realm oidc.SigningRealm, roles []string, expiresAt int64) string {
	t.Helper()
	token, err := realm.Token(oidc.TokenRequest{Subject: "member-1", RealmRoles: roles, Audience: "mesail-api", AuthorizedParty: "mesail-identity", ExpiresAt: expiresAt})
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestGrantNamesTheDomainAndMemberAndCapsTheTimeAt60Seconds(t *testing.T) {
	realm, checker := tokenRealm(t)
	now := time.Now()
	grant, err := checker.Grant(signedToken(t, realm, []string{"mesail-member"}, now.Unix()+3600), now)
	want := time.Unix(now.Unix()+60, 0).UTC().Format(time.RFC3339)
	if err != nil || grant.DomainId != "example.com" || grant.MemberId != "member-1" || grant.ExpiresAt != want {
		t.Fatalf("got %+v, %v, want expiry %s", grant, err, want)
	}
}

func TestGrantEndsWhenTheTokenEndsBefore60Seconds(t *testing.T) {
	realm, checker := tokenRealm(t)
	now := time.Now()
	grant, err := checker.Grant(signedToken(t, realm, []string{"mesail-member"}, now.Unix()+20), now)
	if err != nil || grant.ExpiresAt != time.Unix(now.Unix()+20, 0).UTC().Format(time.RFC3339) {
		t.Fatalf("got %+v, %v", grant, err)
	}
}

func TestGrantRefusesAServiceToken(t *testing.T) {
	realm, checker := tokenRealm(t)
	now := time.Now()
	grant, err := checker.Grant(signedToken(t, realm, []string{"mesail-service"}, now.Unix()+60), now)
	if !errors.Is(err, Unauthenticated{}) || grant != (Grant{}) {
		t.Fatalf("got %+v, %v", grant, err)
	}
}

func TestGrantRefusesAnExpiredToken(t *testing.T) {
	realm, checker := tokenRealm(t)
	now := time.Now()
	if _, err := checker.Grant(signedToken(t, realm, []string{"mesail-member"}, now.Unix()-120), now); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestGrantRefusesAForgedToken(t *testing.T) {
	_, checker := tokenRealm(t)
	if _, err := checker.Grant("eyJhbGciOiJub25lIn0.eyJzdWIiOiJhbmEifQ.", time.Now()); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}
