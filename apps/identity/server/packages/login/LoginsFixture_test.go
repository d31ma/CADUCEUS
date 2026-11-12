package login

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"mesail.dev/packages/oidc"
)

type fakeRealm struct {
	events     *[]string
	session    oidc.KeycloakSession
	refreshed  oidc.KeycloakSession
	content    oidc.SessionContent
	signInErr  error
	refreshErr error
	openErr    error
	sealErr    error
	usernames  []string
	signedOut  []string
	sealed     []oidc.SessionContent
}

func (realm *fakeRealm) SignIn(username string, password string, oneTimeCode string) (oidc.KeycloakSession, error) {
	*realm.events = append(*realm.events, "sign-in "+username)
	realm.usernames = append(realm.usernames, username)
	return realm.session, realm.signInErr
}

func (realm *fakeRealm) Refresh(refreshToken string) (oidc.KeycloakSession, error) {
	*realm.events = append(*realm.events, "refresh "+refreshToken)
	return realm.refreshed, realm.refreshErr
}

func (realm *fakeRealm) SignOut(refreshToken string) error {
	*realm.events = append(*realm.events, "sign-out "+refreshToken)
	realm.signedOut = append(realm.signedOut, refreshToken)
	return nil
}

func (realm *fakeRealm) Seal(content oidc.SessionContent) (string, error) {
	*realm.events = append(*realm.events, "seal")
	realm.sealed = append(realm.sealed, content)
	return "sealed-key", realm.sealErr
}

func (realm *fakeRealm) Open(sessionKey string) (oidc.SessionContent, error) {
	*realm.events = append(*realm.events, "open "+sessionKey)
	return realm.content, realm.openErr
}

func (realm *fakeRealm) PasskeyChallenge() (oidc.PasskeyChallenge, error) {
	*realm.events = append(*realm.events, "passkey challenge")
	return oidc.PasskeyChallenge{OptionsJson: "{}", ChallengeKey: "challenge-key"}, nil
}

func (realm *fakeRealm) PasskeySignIn(challengeKey string, credentialJson string) (oidc.KeycloakSession, error) {
	*realm.events = append(*realm.events, "passkey "+challengeKey)
	return realm.session, realm.signInErr
}

type fakePeople struct {
	events *[]string
	people map[string]Person
	err    error
}

func (people *fakePeople) Person(personId string) (Person, error) {
	*people.events = append(*people.events, "person "+personId)
	if people.err != nil {
		return Person{}, people.err
	}
	return people.found(personId), nil
}

func (people *fakePeople) found(personId string) Person {
	person, known := people.people[personId]
	if !known {
		return Person{PersonId: personId}
	}
	return person
}

type fakeVerifier struct {
	claims oidc.Claims
}

func (verifier fakeVerifier) Verify(rawToken string, now time.Time) (oidc.Claims, error) {
	return verifier.claims, nil
}

type loginWorld struct {
	events []string
	realm  *fakeRealm
	people *fakePeople
	signer oidc.TokenSigner
}

func newLoginWorld(t *testing.T) *loginWorld {
	t.Helper()
	world := &loginWorld{signer: testSigner(t)}
	world.realm = &fakeRealm{events: &world.events, session: anaSession(), refreshed: anaRefreshed(), content: anaContent()}
	world.people = &fakePeople{events: &world.events, people: map[string]Person{"person-1": anaPerson()}}
	return world
}

func (world *loginWorld) logins() Logins {
	return Logins{Realm: world.realm, People: world.people, Signer: world.signer}
}

func (world *loginWorld) setPerson(person Person) {
	world.people.people["person-1"] = person
}

func (world *loginWorld) lastSealed(t *testing.T) oidc.SessionContent {
	t.Helper()
	if len(world.realm.sealed) == 0 {
		t.Fatalf("nothing was sealed")
	}
	return world.realm.sealed[len(world.realm.sealed)-1]
}

func anaSession() oidc.KeycloakSession {
	return oidc.KeycloakSession{PersonId: "person-1", SessionId: "session-1", Username: "ana", DisplayName: "Ana Realm", RefreshToken: "refresh-1"}
}

func anaRefreshed() oidc.KeycloakSession {
	return oidc.KeycloakSession{PersonId: "person-1", SessionId: "session-1", Username: "ana", DisplayName: "Ana Realm", RefreshToken: "refresh-2"}
}

func anaContent() oidc.SessionContent {
	return oidc.SessionContent{Domain: "example.com", RefreshToken: "refresh-1", PersonId: "person-1"}
}

func anaPerson() Person {
	return Person{PersonId: "person-1", Username: "ana", DisplayName: "Ana Example", DomainIds: []string{"example.com", "example.org"}, AdminDomainIds: []string{"example.org"}, DefaultDomainId: "example.org", State: "active"}
}

func withState(person Person, state string) Person {
	person.State = state
	return person
}

func withDomains(person Person, domainIds []string, defaultDomainId string) Person {
	person.DomainIds = domainIds
	person.DefaultDomainId = defaultDomainId
	return person
}

func testDocument(t *testing.T, issuer string) string {
	t.Helper()
	key, err := oidc.SigningKey{}.Generated()
	failOn(t, err)
	text, err := key.Text()
	failOn(t, err)
	document := map[string]any{"yon": map[string]any{
		"oidc":      map[string]any{"issuer": issuer, "jwks": "https://identity.test/jwks", "audience": "mesail-api", "keycloak": "https://keycloak.test/realms/mesail/protocol/openid-connect", "admin": "https://keycloak.test/admin/realms/mesail"},
		"identity":  map[string]any{"issuer": "https://identity.test", "signing": map[string]any{"key": text}},
		"service":   map[string]any{"client": map[string]any{"secret": "service-secret"}},
		"directory": map[string]any{"url": "https://directory.test"},
	}}
	raw, err := json.Marshal(document)
	failOn(t, err)
	return string(raw)
}

func testSigner(t *testing.T) oidc.TokenSigner {
	t.Helper()
	signer := oidc.TokenSignerFactory{}.FromJson(testDocument(t, "https://identity.test"))
	failOn(t, signer.Problem)
	return signer
}

func brokenSigner() oidc.TokenSigner {
	return oidc.TokenSigner{Problem: oidc.NotConfigured{}}
}

func signedClaims(t *testing.T, token string) oidc.Claims {
	t.Helper()
	parsed, err := oidc.TokenParser{}.Parse(token)
	failOn(t, err)
	return parsed.Claims
}

func failOn(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func isError(t *testing.T, got error, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
}

func same(t *testing.T, name string, got any, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}
