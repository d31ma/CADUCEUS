package channels

import (
	"testing"
	"time"

	"mesail.dev/packages/oidc"
)

func signingRealm(t *testing.T) oidc.SigningRealm {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	realm, err := oidc.SigningRealm{}.StartedMesail()
	wantNoError(t, err)
	document, err := realm.ApplicationJson()
	wantNoError(t, err)
	t.Setenv("YON_APPLICATION_JSON", document)
	wantNoError(t, realm.Publish(time.Now()))
	return realm
}

func bearer(t *testing.T, realm oidc.SigningRealm, subject string, roles ...string) string {
	t.Helper()
	token, err := realm.Token(oidc.TokenRequest{
		Subject:         subject,
		Name:            subject,
		Email:           subject + "@example.com",
		RealmRoles:      roles,
		Domain:          "example.com",
		Audience:        "mesail-api",
		AuthorizedParty: "mesail-web",
		ExpiresAt:       time.Now().Unix() + 60,
	})
	wantNoError(t, err)
	return "Bearer " + token
}

func TestAMemberTokenNamesTheCallerAndTheDomain(t *testing.T) {
	realm := signingRealm(t)
	caller, err := TokenCallers{}.Caller(bearer(t, realm, "ana", "mesail-member"))
	wantNoError(t, err)
	wantEqual(t, caller.DomainId, "example.com")
	wantEqual(t, caller.MemberId, "ana")
	wantEqual(t, caller.Roles, []string{"Member"})
}

func TestADomainAdminTokenIsAccepted(t *testing.T) {
	realm := signingRealm(t)
	caller, err := TokenCallers{}.Caller(bearer(t, realm, "dee", "mesail-admin"))
	wantNoError(t, err)
	wantEqual(t, caller.Roles, []string{"DomainAdmin"})
}

func TestATokenWithoutAMemberRoleIsForbidden(t *testing.T) {
	realm := signingRealm(t)
	_, err := TokenCallers{}.Caller(bearer(t, realm, "relay", "mesail-service"))
	wantError[*Forbidden](t, err)
}

func TestAForgedTokenIsUnauthenticated(t *testing.T) {
	signingRealm(t)
	_, err := TokenCallers{}.Caller("Bearer forged.token.value")
	wantError[*Unauthenticated](t, err)
}
