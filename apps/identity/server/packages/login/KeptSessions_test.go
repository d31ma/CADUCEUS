package login

import (
	"net/http"
	"testing"

	"mesail.dev/packages/oidc"
)

func endedAt(person Person, sessionsEndedAt string) Person {
	person.SessionsEndedAt = sessionsEndedAt
	return person
}

func signedInAt(signedIn string) oidc.SessionContent {
	content := anaContent()
	content.SignedInAt = signedIn
	return content
}

func TestKeptSignInAsksForAKeptSessionAndSealsIt(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.logins().SignIn("ana", "right-password", "", true)
	failOn(t, err)
	same(t, "kept asked", world.realm.kept, []bool{true})
	same(t, "kept", grant.Kept, true)
	same(t, "sealed", world.lastSealed(t), oidc.SessionContent{Domain: "example.org", RefreshToken: "refresh-1", PersonId: "person-1", Kept: true, SignedInAt: signedInText()})
}

func TestKeptPasskeySignInAsksForAKeptSession(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.logins().PasskeySignIn("challenge-key", "{}", "", true)
	failOn(t, err)
	same(t, "kept asked", world.realm.kept, []bool{true})
	same(t, "kept", grant.Kept, true)
}

func TestRefreshKeepsTheKeptFlagAndTheSignInTime(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.content = oidc.SessionContent{Domain: "example.com", RefreshToken: "refresh-1", PersonId: "person-1", Kept: true, SignedInAt: "2026-11-01T08:00:00Z"}
	grant, err := world.logins().Refresh("key")
	failOn(t, err)
	same(t, "kept", grant.Kept, true)
	same(t, "sealed", world.lastSealed(t), oidc.SessionContent{Domain: "example.com", RefreshToken: "refresh-2", PersonId: "person-1", Kept: true, SignedInAt: "2026-11-01T08:00:00Z"})
}

func TestRefreshOfASessionStartedBeforeTheEndIsRevoked(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(endedAt(anaPerson(), "2026-11-10T09:00:00.5Z"))
	world.realm.content = signedInAt("2026-11-10T09:00:00.4Z")
	_, err := world.logins().Refresh("key")
	isError(t, err, AccessRevoked{})
	same(t, "events", world.events, []string{"open key", "person person-1"})
}

func TestRefreshOfASessionStartedAfterTheEndWorks(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(endedAt(anaPerson(), "2026-11-10T09:00:00.5Z"))
	world.realm.content = signedInAt("2026-11-10T09:00:00.6Z")
	_, err := world.logins().Refresh("key")
	failOn(t, err)
}

func TestRefreshOfASessionStartedAtTheEndWorks(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(endedAt(anaPerson(), "2026-11-10T09:00:00.5Z"))
	world.realm.content = signedInAt("2026-11-10T09:00:00.5Z")
	_, err := world.logins().Refresh("key")
	failOn(t, err)
}

func TestRefreshOfAnOlderKeyWithoutASignInTimeIsRevokedOnceSessionsEnded(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(endedAt(anaPerson(), "2026-11-10T09:00:00Z"))
	_, err := world.logins().Refresh("key")
	isError(t, err, AccessRevoked{})
}

func TestRefreshIsRevokedWhenTheEndTimeCannotBeRead(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(endedAt(anaPerson(), "yesterday"))
	world.realm.content = signedInAt(signedInText())
	_, err := world.logins().Refresh("key")
	isError(t, err, AccessRevoked{})
}

func TestSwitchOfASessionStartedBeforeTheEndIsRevoked(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(endedAt(anaPerson(), "2026-11-10T09:00:00Z"))
	world.realm.content = signedInAt("2026-11-09T09:00:00Z")
	_, err := world.logins().Switch("key", "example.org")
	isError(t, err, AccessRevoked{})
	same(t, "events", world.events, []string{"open key", "person person-1"})
}

func TestRestartedEndsTheOtherSessionsAndKeepsTheDevice(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.logins().Restarted(anaSession(), "example.com", true)
	failOn(t, err)
	same(t, "kept", grant.Kept, true)
	same(t, "sealed", world.lastSealed(t), oidc.SessionContent{Domain: "example.com", RefreshToken: "refresh-1", PersonId: "person-1", Kept: true, SignedInAt: signedInText()})
}

func TestRestartedSignsTheNewSessionOutWhenTheEndCannotBeRecorded(t *testing.T) {
	world := newLoginWorld(t)
	world.people.endedErr = oidc.SessionUnavailable{}
	_, err := world.logins().Restarted(anaSession(), "example.com", true)
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "signed out", world.realm.signedOut, []string{"refresh-1"})
}

func TestSignOutEverywhereRecordsTheEndAndEndsEveryKeycloakSession(t *testing.T) {
	world := newLoginWorld(t)
	keycloak := newFakeKeycloak(t)
	err := world.sessions(keycloak.account()).SignOutEverywhere(world.authorization(t))
	failOn(t, err)
	same(t, "events", world.events, []string{"sessions-ended person-1"})
	same(t, "logged out", keycloak.loggedOut, 1)
	same(t, "revoked", keycloak.revoked, []string{"mesail-identity"})
}

func TestSignOutEverywhereRefusesABadHeader(t *testing.T) {
	world := newLoginWorld(t)
	err := world.sessions(unavailableAccount()).SignOutEverywhere("Basic abc")
	isError(t, err, oidc.MissingBearerToken{})
	same(t, "events", world.events, []string(nil))
}

func TestSignOutEverywhereStopsWhenTheEndCannotBeRecorded(t *testing.T) {
	world := newLoginWorld(t)
	world.people.endedErr = oidc.SessionUnavailable{}
	keycloak := newFakeKeycloak(t)
	err := world.sessions(keycloak.account()).SignOutEverywhere(world.authorization(t))
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "logged out", keycloak.loggedOut, 0)
}

func TestSignOutEverywhereReportsAKeycloakFailure(t *testing.T) {
	world := newLoginWorld(t)
	err := world.sessions(unavailableAccount()).SignOutEverywhere(world.authorization(t))
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "events", world.events, []string{"sessions-ended person-1"})
}

func TestKeptPasswordChangeAsksKeycloakForAKeptSession(t *testing.T) {
	world := newLoginWorld(t)
	keycloak := newFakeKeycloak(t)
	grant, err := world.sessions(keycloak.account()).ChangePassword(world.authorization(t), PasswordChange{CurrentPassword: "right-password", NewPassword: "a-new-long-password", Kept: true})
	failOn(t, err)
	same(t, "sign-in scope", keycloak.scopes[len(keycloak.scopes)-1], "openid offline_access")
	same(t, "kept", grant.Kept, true)
	same(t, "ended recorded", world.events[0], "sessions-ended person-1")
}

func TestDirectoryPeopleRecordsTheEndOfTheSessions(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusNoContent, "")
	err := directory.people(t).EndSessions("person-1")
	failOn(t, err)
	same(t, "path", directory.path, "/api/v1/directory/people/person-1/sessions-ended")
}

func TestDirectoryPeopleAcceptsAnUnknownPersonWhenEndingSessions(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusNotFound, "")
	err := directory.people(t).EndSessions("person-1")
	failOn(t, err)
}

func TestDirectoryPeopleReportsARefusedEndOfSessions(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusForbidden, "")
	err := directory.people(t).EndSessions("person-1")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestDirectoryPeopleReportsAnUnreachableDirectoryWhenEndingSessions(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusNoContent, "")
	people := directory.people(t)
	directory.server.Close()
	err := people.EndSessions("person-1")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestDirectoryPeopleReadsTheEndOfTheSessions(t *testing.T) {
	directory := newDirectoryServer(t, http.StatusOK, `{"personId":"person-1","state":"active","sessionsEndedAt":"2026-11-10T09:00:00Z"}`)
	person, err := directory.people(t).Person("person-1")
	failOn(t, err)
	same(t, "ended", person.SessionsEndedAt, "2026-11-10T09:00:00Z")
}
