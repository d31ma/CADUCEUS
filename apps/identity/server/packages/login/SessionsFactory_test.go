package login

import (
	"testing"
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

func isLegacy(sessions Sessions) bool {
	_, legacy := sessions.(LegacySessions)
	return legacy
}

func TestSessionsFactoryChoosesLegacySessionsForAPerRealmIssuer(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	sessions := SessionsFactory{}.FromJson(testDocument(t, "https://keycloak.test/realms/{realm}"))
	same(t, "legacy", isLegacy(sessions), true)
}

func TestSessionsFactoryChoosesLegacySessionsForIncompleteSettings(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	sessions := SessionsFactory{}.FromJson(testDocument(t, ""))
	same(t, "legacy", isLegacy(sessions), true)
}

func TestSessionsFactoryChoosesLegacySessionsForBrokenJson(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	sessions := SessionsFactory{}.FromJson("{")
	same(t, "legacy", isLegacy(sessions), true)
}

func TestSessionsFactoryChoosesRealmSessionsForOneIssuer(t *testing.T) {
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
