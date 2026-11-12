//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
)

type passkeyChallenge struct {
	OptionsJson  string `json:"optionsJson"`
	ChallengeKey string `json:"challengeKey"`
}

type passkeySetup struct {
	OptionsJson string `json:"optionsJson"`
	SetupKey    string `json:"setupKey"`
}

type passkey struct {
	PasskeyId  string `json:"passkeyId"`
	Label      string `json:"label"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt"`
}

type passkeyOwner struct {
	Person        newPerson
	Session       session
	Authenticator *Authenticator
	Passkey       passkey
}

func passkeySetupReply(t *testing.T, stack Stack, token string, currentPassword string) Reply {
	t.Helper()
	request := map[string]string{"currentPassword": currentPassword}
	return stack.Call(t, http.MethodPost, "/api/v1/identity/passkeys/setup", token, request)
}

func startPasskeySetup(t *testing.T, stack Stack, token string, currentPassword string) passkeySetup {
	t.Helper()
	var setup passkeySetup
	passkeySetupReply(t, stack, token, currentPassword).Expect(t, http.StatusOK).Decode(t, &setup)
	return setup
}

func confirmPasskeyReply(t *testing.T, stack Stack, token string, setupKey string, credentialJson string, label string) Reply {
	t.Helper()
	request := map[string]string{"setupKey": setupKey, "credentialJson": credentialJson, "label": label}
	return stack.Call(t, http.MethodPost, "/api/v1/identity/passkeys", token, request)
}

func addPasskey(t *testing.T, stack Stack, token string, currentPassword string, label string) (*Authenticator, passkey) {
	t.Helper()
	setup := startPasskeySetup(t, stack, token, currentPassword)
	authenticator := NewAuthenticator(t)
	credentialJson := authenticator.Create(t, setup.OptionsJson)
	var added passkey
	confirmPasskeyReply(t, stack, token, setup.SetupKey, credentialJson, label).Expect(t, http.StatusCreated).Decode(t, &added)
	return authenticator, added
}

func listPasskeys(t *testing.T, stack Stack, token string) []passkey {
	t.Helper()
	var list []passkey
	stack.Call(t, http.MethodGet, "/api/v1/identity/passkeys", token, nil).Expect(t, http.StatusOK).Decode(t, &list)
	return list
}

func removePasskeyReply(t *testing.T, stack Stack, token string, passkeyId string, currentPassword string) Reply {
	t.Helper()
	request := map[string]string{"currentPassword": currentPassword}
	return stack.Call(t, http.MethodDelete, "/api/v1/identity/passkeys/"+passkeyId, token, request)
}

func passkeyChallengeFor(t *testing.T, stack Stack) passkeyChallenge {
	t.Helper()
	var challenge passkeyChallenge
	stack.Call(t, http.MethodPost, "/api/v1/identity/passkeys/challenge", "", nil).Expect(t, http.StatusOK).Decode(t, &challenge)
	return challenge
}

func passkeySignInReply(t *testing.T, stack Stack, request map[string]any) Reply {
	t.Helper()
	return stack.Call(t, http.MethodPost, "/api/v1/identity/session/passkey", "", request)
}

func passkeySignInRequest(challenge passkeyChallenge, credentialJson string) map[string]any {
	return map[string]any{"challengeKey": challenge.ChallengeKey, "credentialJson": credentialJson}
}

func signInWithPasskey(t *testing.T, stack Stack, authenticator *Authenticator) session {
	t.Helper()
	challenge := passkeyChallengeFor(t, stack)
	request := passkeySignInRequest(challenge, authenticator.Assert(t, challenge.OptionsJson))
	var signedIn session
	passkeySignInReply(t, stack, request).Expect(t, http.StatusOK).Decode(t, &signedIn)
	return signedIn
}

func newPasskeyOwner(t *testing.T, stack Stack) passkeyOwner {
	t.Helper()
	person := changedPassword(t, stack, createPerson(t, stack, fixture(t, stack, "ANA").AccessToken))
	signedIn := signIn(t, stack, person.Username, person.Password)
	authenticator, added := addPasskey(t, stack, signedIn.AccessToken, person.Password, "Mac")
	return passkeyOwner{Person: person, Session: signedIn, Authenticator: authenticator, Passkey: added}
}

func passkeyIds(list []passkey) []string {
	ids := []string{}
	for _, item := range list {
		ids = append(ids, item.PasskeyId)
	}
	return ids
}

func containsText(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func addMorePasskeys(t *testing.T, stack Stack, owner passkeyOwner, count int) {
	t.Helper()
	for index := 0; index < count; index++ {
		addPasskey(t, stack, owner.Session.AccessToken, owner.Person.Password, "Phone")
	}
}

func excludedIds(options creationOptions) []string {
	ids := []string{}
	for _, reference := range options.ExcludeCredentials {
		ids = append(ids, reference.Id)
	}
	return ids
}

func renamePasskeyReply(t *testing.T, stack Stack, token string, passkeyId string, label string) Reply {
	t.Helper()
	request := map[string]string{"label": label}
	return stack.Call(t, http.MethodPatch, "/api/v1/identity/passkeys/"+passkeyId, token, request)
}
