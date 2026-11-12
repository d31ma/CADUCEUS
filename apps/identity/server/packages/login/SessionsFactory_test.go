package login

import (
	"errors"
	"testing"

	"mesail.dev/packages/oidc"
)

func realmSessionsFrom(t *testing.T, sessions Sessions) RealmSessions {
	t.Helper()
	realm, isRealm := sessions.(RealmSessions)
	if !isRealm {
		t.Fatalf("sessions = %T, want RealmSessions", sessions)
	}
	return realm
}

func directoryUrl(t *testing.T, sessions RealmSessions) string {
	t.Helper()
	people, isDirectory := sessions.Logins.People.(DirectoryPeople)
	if !isDirectory {
		t.Fatalf("people = %T, want DirectoryPeople", sessions.Logins.People)
	}
	return people.BaseUrl
}

func unavailable(t *testing.T, sessions Sessions) {
	t.Helper()
	_, err := sessions.SignIn("ana", "password", "")
	same(t, "unavailable", errors.Is(err, oidc.SessionUnavailable{}), true)
}

func TestSessionsFactoryWithIncompleteSettingsIsUnavailable(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	unavailable(t, SessionsFactory{}.FromJson(testDocument(t, "")))
}

func TestSessionsFactoryWithBrokenJsonIsUnavailable(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	unavailable(t, SessionsFactory{}.FromJson("{"))
}

func TestSessionsFactoryReadsTheDirectoryUrl(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	sessions := realmSessionsFrom(t, SessionsFactory{}.FromJson(testDocument(t, "https://identity.test")))
	same(t, "directory", directoryUrl(t, sessions), "https://directory.test")
}

func TestSessionsFactoryReadsTheEnvironment(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("YON_APPLICATION_JSON", testDocument(t, "https://identity.test"))
	sessions := realmSessionsFrom(t, SessionsFactory{}.Configured())
	same(t, "directory", directoryUrl(t, sessions), "https://directory.test")
}

func TestSessionsFactoryReadsTheServiceSecret(t *testing.T) {
	issuer := SessionsFactory{}.ServiceTokens(testDocument(t, "https://identity.test"))
	same(t, "secret", issuer.Check.Client.Secret, "service-secret")
	same(t, "accepts", issuer.Check.Accepts("mesail-service", "service-secret"), true)
}

func TestSessionsFactoryReadsTheServiceSecretFromTheEnvironment(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", testDocument(t, "https://identity.test"))
	issuer := SessionsFactory{}.ConfiguredServiceTokens()
	same(t, "secret", issuer.Check.Client.Secret, "service-secret")
}
