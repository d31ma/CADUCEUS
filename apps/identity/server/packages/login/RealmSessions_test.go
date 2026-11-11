package login

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mesail.dev/packages/oidc"
)

type fakeKeycloak struct {
	server  *httptest.Server
	ended   []string
	resets  int
	updates int
}

func newFakeKeycloak(t *testing.T) *fakeKeycloak {
	t.Helper()
	keycloak := &fakeKeycloak{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", keycloak.answer(`{"access_token":"keycloak-access","refresh_token":"refresh-new","expires_in":300}`))
	mux.HandleFunc("PUT /admin/users/person-1/reset-password", keycloak.reset)
	mux.HandleFunc("GET /admin/users/person-1", keycloak.answer(`{"id":"person-1","attributes":{"mesail.mustChangePassword":["true"]}}`))
	mux.HandleFunc("PUT /admin/users/person-1", keycloak.update)
	mux.HandleFunc("GET /admin/users/person-1/sessions", keycloak.answer(`[{"id":"session-1"},{"id":"session-2"}]`))
	mux.HandleFunc("DELETE /admin/sessions/{id}", keycloak.end)
	keycloak.server = httptest.NewServer(mux)
	t.Cleanup(keycloak.server.Close)
	return keycloak
}

func (keycloak *fakeKeycloak) answer(body string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(body))
	}
}

func (keycloak *fakeKeycloak) reset(writer http.ResponseWriter, request *http.Request) {
	keycloak.resets++
	writer.WriteHeader(http.StatusNoContent)
}

func (keycloak *fakeKeycloak) update(writer http.ResponseWriter, request *http.Request) {
	keycloak.updates++
	writer.WriteHeader(http.StatusNoContent)
}

func (keycloak *fakeKeycloak) end(writer http.ResponseWriter, request *http.Request) {
	keycloak.ended = append(keycloak.ended, request.PathValue("id"))
	writer.WriteHeader(http.StatusNoContent)
}

func (keycloak *fakeKeycloak) account() oidc.RealmAccount {
	issuer := oidc.SessionIssuer{Client: keycloak.server.Client(), Keycloak: keycloak.server.URL, Admin: keycloak.server.URL + "/admin"}
	verifier := fakeVerifier{claims: oidc.Claims{Subject: "person-1", SessionId: "session-1", Username: "ana", Name: "Ana Realm"}}
	return oidc.RealmAccount{Session: oidc.RealmSession{Sessions: issuer, Verifier: verifier, Now: time.Now}, Now: time.Now}
}

func unavailableAccount() oidc.RealmAccount {
	return oidc.RealmAccount{Session: oidc.RealmSession{Sessions: oidc.SessionIssuer{Problem: oidc.NotConfigured{}}, Now: time.Now}, Now: time.Now}
}

func testGate(signer oidc.TokenSigner) oidc.Gate {
	verifier := oidc.ExactIssuerVerifier{Issuer: "https://identity.test", Jwks: "https://identity.test/jwks", Keys: SignerKeys{Signer: signer}, Rules: oidc.MesailClaimRules{Audience: "mesail-api"}}
	return oidc.NewGate(verifier, time.Now)
}

func (world *loginWorld) sessions(account oidc.RealmAccount) RealmSessions {
	return RealmSessions{Logins: world.logins(), Account: account, Gate: testGate(world.signer)}
}

func (world *loginWorld) authorization(t *testing.T) string {
	t.Helper()
	issued, err := world.signer.Sign(oidc.MesailToken{Subject: "person-1", AuthorizedParty: "mesail-web", Domain: "example.com", Username: "ana", SessionId: "session-1", RealmRoles: []string{"mesail-member"}})
	failOn(t, err)
	return "Bearer " + issued.AccessToken
}

func TestRealmSessionsSignsIn(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.sessions(unavailableAccount()).SignIn("ana", "right-password", "")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "example.org")
}

func TestRealmSessionsRefreshes(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.sessions(unavailableAccount()).Refresh("key-1")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "example.com")
}

func TestRealmSessionsSwitches(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.sessions(unavailableAccount()).Switch("key-1", "example.org")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "example.org")
}

func TestRealmSessionsSignsOut(t *testing.T) {
	world := newLoginWorld(t)
	err := world.sessions(unavailableAccount()).SignOut("key-1")
	failOn(t, err)
	same(t, "signed out", world.realm.signedOut, []string{"refresh-1"})
}

func TestRealmSessionsIdentifiesTheCaller(t *testing.T) {
	world := newLoginWorld(t)
	caller, err := world.sessions(unavailableAccount()).Identify(world.authorization(t))
	failOn(t, err)
	same(t, "member", caller.MemberId, "person-1")
	same(t, "domain", caller.DomainId, "example.com")
	same(t, "roles", caller.Roles, []string{"Member"})
}

func TestRealmSessionsChangesThePassword(t *testing.T) {
	world := newLoginWorld(t)
	keycloak := newFakeKeycloak(t)
	grant, err := world.sessions(keycloak.account()).ChangePassword(world.authorization(t), "right-password", "a-new-long-password", "")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "example.com")
	same(t, "person", grant.PersonId, "person-1")
	same(t, "sealed", world.lastSealed(t), oidc.SessionContent{Domain: "example.com", RefreshToken: "refresh-new", PersonId: "person-1"})
	same(t, "resets", keycloak.resets, 1)
	same(t, "ended", keycloak.ended, []string{"session-2"})
}

func TestRealmSessionsChangePasswordRefusesABadHeader(t *testing.T) {
	world := newLoginWorld(t)
	_, err := world.sessions(unavailableAccount()).ChangePassword("Basic abc", "right-password", "a-new-long-password", "")
	isError(t, err, oidc.MissingBearerToken{})
}

func TestRealmSessionsChangePasswordPassesAnAccountFailureThrough(t *testing.T) {
	world := newLoginWorld(t)
	_, err := world.sessions(unavailableAccount()).ChangePassword(world.authorization(t), "right-password", "a-new-long-password", "")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "sealed", world.realm.sealed, []oidc.SessionContent(nil))
}

func TestRealmSessionsTotpEnabledRefusesABadHeader(t *testing.T) {
	world := newLoginWorld(t)
	_, err := world.sessions(unavailableAccount()).TotpEnabled("")
	isError(t, err, oidc.MissingBearerToken{})
}

func TestRealmSessionsTotpEnabledAsksTheAccount(t *testing.T) {
	world := newLoginWorld(t)
	_, err := world.sessions(unavailableAccount()).TotpEnabled(world.authorization(t))
	isError(t, err, oidc.SessionUnavailable{})
}

func TestRealmSessionsSetupTotpRefusesABadHeader(t *testing.T) {
	world := newLoginWorld(t)
	_, err := world.sessions(unavailableAccount()).SetupTotp("", "right-password")
	isError(t, err, oidc.MissingBearerToken{})
}

func TestRealmSessionsSetupTotpAsksTheAccount(t *testing.T) {
	world := newLoginWorld(t)
	_, err := world.sessions(unavailableAccount()).SetupTotp(world.authorization(t), "right-password")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestRealmSessionsConfirmTotpRefusesABadHeader(t *testing.T) {
	world := newLoginWorld(t)
	err := world.sessions(unavailableAccount()).ConfirmTotp("", "setup-key", "123456")
	isError(t, err, oidc.MissingBearerToken{})
}

func TestRealmSessionsConfirmTotpAsksTheAccount(t *testing.T) {
	world := newLoginWorld(t)
	err := world.sessions(unavailableAccount()).ConfirmTotp(world.authorization(t), "setup-key", "123456")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestRealmSessionsRemoveTotpRefusesABadHeader(t *testing.T) {
	world := newLoginWorld(t)
	err := world.sessions(unavailableAccount()).RemoveTotp("", "right-password", "123456")
	isError(t, err, oidc.MissingBearerToken{})
}

func TestRealmSessionsRemoveTotpAsksTheAccount(t *testing.T) {
	world := newLoginWorld(t)
	err := world.sessions(unavailableAccount()).RemoveTotp(world.authorization(t), "right-password", "123456")
	isError(t, err, oidc.SessionUnavailable{})
}
