package devicebox

import (
	"testing"
	"time"

	"mesail.dev/packages/oidc"
)

func signingRealm(t *testing.T) oidc.SigningRealm {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	realm, err := oidc.SigningRealm{}.StartedMesail()
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

func bearer(t *testing.T, realm oidc.SigningRealm, subject string, roles ...string) string {
	t.Helper()
	token, err := realm.Token(oidc.TokenRequest{Subject: subject, Name: subject, Email: subject + "@example.com", RealmRoles: roles, Domain: "example.com", Audience: "mesail-api", AuthorizedParty: "mesail-web", ExpiresAt: time.Now().Unix() + 60})
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + token
}

func TestMembersAndAdminsMayUseTheMailboxButOtherRolesMayNot(t *testing.T) {
	realm := signingRealm(t)
	for _, role := range []string{"mesail-member", "mesail-admin"} {
		caller, err := TokenCallers{}.Member(bearer(t, realm, "ana", role))
		if err != nil || caller.MemberId != "ana" || caller.DomainId != "example.com" {
			t.Fatalf("%s got %+v %v", role, caller, err)
		}
	}
	if _, err := (TokenCallers{}).Member(bearer(t, realm, "job", "mesail-service")); numberOf(err) != 6002 {
		t.Fatalf("got %v", err)
	}
	if _, err := (TokenCallers{}).Member("Bearer forged"); numberOf(err) != 6001 {
		t.Fatalf("got %v", err)
	}
	if caller, err := (TokenCallers{}).Service(bearer(t, realm, "job", "mesail-service")); err != nil || caller.DomainId != "example.com" {
		t.Fatalf("got %+v %v", caller, err)
	}
}
