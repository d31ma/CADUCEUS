package oidc

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

func (realm *mesailRealm) handlePasskeys(admin *http.ServeMux) {
	admin.HandleFunc("GET /admin/realms/mesail/mesail/users/person-1/passkeys", realm.listPasskeys)
	admin.HandleFunc("POST /admin/realms/mesail/mesail/users/person-1/passkeys", realm.addPasskey)
	admin.HandleFunc("PUT /admin/realms/mesail/mesail/users/person-1/passkeys/{id}", realm.renamePasskey)
	admin.HandleFunc("DELETE /admin/realms/mesail/mesail/users/person-1/passkeys/{id}", realm.removePasskey)
}

func (realm *mesailRealm) listPasskeys(writer http.ResponseWriter, request *http.Request) {
	_ = json.NewEncoder(writer).Encode(realm.passkeys)
}

func (realm *mesailRealm) addPasskey(writer http.ResponseWriter, request *http.Request) {
	added := PasskeyAddRequest{}
	_ = json.NewDecoder(request.Body).Decode(&added)
	realm.added = append(realm.added, added)
	if added.CredentialJson == "too-many" {
		passkeyProblem(writer, http.StatusConflict, "too_many")
		return
	}
	record := PasskeyRecord{PasskeyId: "passkey-new", CredentialId: "credential-new", Label: added.Label, CreatedAt: 1762905600000}
	realm.passkeys = append(realm.passkeys, record)
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(record)
}

func (realm *mesailRealm) renamePasskey(writer http.ResponseWriter, request *http.Request) {
	renamed := PasskeyLabelRequest{}
	_ = json.NewDecoder(request.Body).Decode(&renamed)
	switch {
	case request.PathValue("id") != "passkey-1":
		passkeyProblem(writer, http.StatusNotFound, "not_found")
	case renamed.Label == "Phone":
		passkeyProblem(writer, http.StatusConflict, "label_exists")
	case strings.Contains(renamed.Label, "\t"):
		passkeyProblem(writer, http.StatusBadRequest, "invalid_request")
	default:
		_ = json.NewEncoder(writer).Encode(PasskeyRecord{PasskeyId: "passkey-1", Label: renamed.Label, CreatedAt: 1762905600000})
	}
}

func (realm *mesailRealm) removePasskey(writer http.ResponseWriter, request *http.Request) {
	index := slices.IndexFunc(realm.passkeys, func(record PasskeyRecord) bool { return record.PasskeyId == request.PathValue("id") })
	if index < 0 {
		passkeyProblem(writer, http.StatusNotFound, "not_found")
		return
	}
	realm.passkeys = slices.Delete(realm.passkeys, index, index+1)
	writer.WriteHeader(http.StatusNoContent)
}

func (realm *mesailRealm) passkeyGrant(writer http.ResponseWriter, form url.Values) {
	realm.grants = append(realm.grants, form)
	if form.Get("passkey_assertion") != passkeyAssertionFor("person-1") {
		realm.refused(writer)
		return
	}
	realm.granted(writer, "refresh-1")
}

func passkeyProblem(writer http.ResponseWriter, status int, code string) {
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": code})
}

func passkeyAssertionFor(personId string) string {
	return `{"type":"public-key","response":{"userHandle":"` + base64.RawURLEncoding.EncodeToString([]byte(personId)) + `"}}`
}

func passkeyRecord(passkeyId string, label string) PasskeyRecord {
	return PasskeyRecord{PasskeyId: passkeyId, CredentialId: "credential-" + passkeyId, Label: label, CreatedAt: 1762905600000}
}

func (realm *mesailRealm) withPasskeys(count int) {
	for index := range count {
		realm.passkeys = append(realm.passkeys, passkeyRecord("passkey-"+string(rune('1'+index)), "Mac "+string(rune('1'+index))))
	}
}

func TestPasskeyAccountListsWithoutCredentialIdsAndWithRfc3339Times(t *testing.T) {
	realm := newMesailRealm(t)
	used := int64(1762909200000)
	realm.passkeys = []PasskeyRecord{{PasskeyId: "passkey-1", CredentialId: "secret-id", Label: "Mac", CreatedAt: 1762905600000, LastUsedAt: &used}}
	list, err := realm.account(t).Passkeys().List(realmCaller())
	realmIs(t, err, nil)
	realmEqual(t, "passkey", list[0], Passkey{PasskeyId: "passkey-1", Label: "Mac", CreatedAt: "2025-11-12T00:00:00Z", LastUsedAt: "2025-11-12T01:00:00Z"})
}

func TestPasskeyAccountSetupChecksThePasswordAndExcludesThePasskeys(t *testing.T) {
	realm := newMesailRealm(t)
	realm.passkeys = []PasskeyRecord{passkeyRecord("passkey-1", "Mac")}
	setup, err := realm.account(t).Passkeys().Setup(realmCaller(), "right-password", "")
	realmIs(t, err, nil)
	options := PasskeyCreationOptions{}
	realmIs(t, json.Unmarshal([]byte(setup.OptionsJson), &options), nil)
	realmEqual(t, "relying party", options.RelyingParty.Id, "mail.mesail.test")
	realmEqual(t, "user handle", options.User.Id, base64.RawURLEncoding.EncodeToString([]byte("person-1")))
	realmEqual(t, "excluded", options.ExcludeCredentials[0].Id, "credential-passkey-1")
	realmEqual(t, "session signed out", realm.signOuts, 1)
}

func TestPasskeyAccountSetupRefusesAWrongPassword(t *testing.T) {
	realm := newMesailRealm(t)
	_, err := realm.account(t).Passkeys().Setup(realmCaller(), "wrong", "")
	realmIs(t, err, SignInRefused{})
}

func TestPasskeyAccountSetupRefusesAnEleventhPasskey(t *testing.T) {
	realm := newMesailRealm(t)
	realm.withPasskeys(10)
	_, err := realm.account(t).Passkeys().Setup(realmCaller(), "right-password", "")
	realmIs(t, err, TooManyPasskeys{})
}

func TestPasskeyAccountConfirmSendsTheSealedChallengeAndANumberedLabel(t *testing.T) {
	realm := newMesailRealm(t)
	realm.passkeys = []PasskeyRecord{passkeyRecord("passkey-1", "Mac")}
	account := realm.account(t).Passkeys()
	setup, _ := account.Setup(realmCaller(), "right-password", "")
	added, err := account.Confirm(realmCaller(), setup.SetupKey, `{"id":"new"}`, "Mac")
	realmIs(t, err, nil)
	options := PasskeyCreationOptions{}
	_ = json.Unmarshal([]byte(setup.OptionsJson), &options)
	realmEqual(t, "label", added.Label, "Mac 2")
	realmEqual(t, "challenge", realm.added[0].Challenge, options.Challenge)
	realmList(t, "origins", realm.added[0].Origins, []string{"android:apk-key-hash:abc", "https://mail.mesail.test"})
}

func TestPasskeyAccountConfirmAnswersInvalidPasskeyForEveryKeycloakRefusal(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t).Passkeys()
	setup, _ := account.Setup(realmCaller(), "right-password", "")
	_, err := account.Confirm(realmCaller(), setup.SetupKey, "too-many", "Mac")
	realmIs(t, err, InvalidPasskey{})
}

func TestPasskeyAccountConfirmRefusesAnotherPersonsSetup(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t).Passkeys()
	setup, _ := account.Setup(realmCaller(), "right-password", "")
	_, err := account.Confirm(Caller{MemberId: "person-2"}, setup.SetupKey, `{"id":"new"}`, "Mac")
	realmIs(t, err, InvalidPasskey{})
}

func TestPasskeyAccountConfirmAnswersSetupExpiredAfterTenMinutes(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t)
	setup, _ := account.Passkeys().Setup(realmCaller(), "right-password", "")
	account.Now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	_, err := account.Passkeys().Confirm(realmCaller(), setup.SetupKey, `{"id":"new"}`, "Mac")
	realmIs(t, err, SetupExpired{})
	realmEqual(t, "added", len(realm.added), 0)
}

func TestPasskeyAccountRenameMapsKeycloakAnswers(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t).Passkeys()
	renamed, err := account.Rename(realmCaller(), "passkey-1", "Work laptop")
	realmIs(t, err, nil)
	realmEqual(t, "label", renamed.Label, "Work laptop")
	_, takenErr := account.Rename(realmCaller(), "passkey-1", "Phone")
	_, missingErr := account.Rename(realmCaller(), "passkey-9", "Mine")
	_, invalidErr := account.Rename(realmCaller(), "passkey-1", "a\tb")
	_, longErr := account.Rename(realmCaller(), "passkey-1", strings.Repeat("a", 65))
	realmIs(t, takenErr, LabelExists{})
	realmIs(t, missingErr, PasskeyNotFound{})
	realmIs(t, invalidErr, InvalidLabel{})
	realmIs(t, longErr, InvalidLabel{})
}

func TestPasskeyAccountRemoveEndsTheOtherSessions(t *testing.T) {
	realm := newMesailRealm(t)
	realm.passkeys = []PasskeyRecord{passkeyRecord("passkey-1", "Mac")}
	checked, err := realm.account(t).Passkeys().Remove(realmCaller(), "passkey-1", "right-password", "")
	realmIs(t, err, nil)
	realmEqual(t, "kept session", checked.SessionId, "session-a")
	realmList(t, "ended", realm.ended, []string{"session-b"})
	realmEqual(t, "passkeys left", len(realm.passkeys), 0)
}

func TestPasskeyAccountRemoveOfAnUnknownPasskeyClosesTheCheckSession(t *testing.T) {
	realm := newMesailRealm(t)
	_, err := realm.account(t).Passkeys().Remove(realmCaller(), "passkey-9", "right-password", "")
	realmIs(t, err, PasskeyNotFound{})
	realmEqual(t, "session signed out", realm.signOuts, 1)
	realmEqual(t, "ended", len(realm.ended), 0)
}

func TestPasskeyAccountRemoveNeedsTheCurrentPassword(t *testing.T) {
	realm := newMesailRealm(t)
	realm.passkeys = []PasskeyRecord{passkeyRecord("passkey-1", "Mac")}
	_, err := realm.account(t).Passkeys().Remove(realmCaller(), "passkey-1", "wrong", "")
	realmIs(t, err, SignInRefused{})
	realmEqual(t, "passkeys left", len(realm.passkeys), 1)
}

func TestRealmSessionSignsInWithAPasskey(t *testing.T) {
	realm := newMesailRealm(t)
	session := realm.session(t)
	challenge, err := session.PasskeyChallenge()
	realmIs(t, err, nil)
	opened, err := session.PasskeySignIn(challenge.ChallengeKey, passkeyAssertionFor("person-1"))
	realmIs(t, err, nil)
	options := PasskeyRequestOptions{}
	_ = json.Unmarshal([]byte(challenge.OptionsJson), &options)
	realmEqual(t, "person", opened.PersonId, "person-1")
	realmEqual(t, "challenge", realm.grants[0].Get("passkey_challenge"), options.Challenge)
	realmEqual(t, "origins", realm.grants[0].Get("passkey_origins"), "android:apk-key-hash:abc https://mail.mesail.test")
	realmEqual(t, "scope", realm.grants[0].Get("scope"), "openid")
}

func TestRealmSessionRefusesAWrongPasskey(t *testing.T) {
	realm := newMesailRealm(t)
	session := realm.session(t)
	challenge, _ := session.PasskeyChallenge()
	_, err := session.PasskeySignIn(challenge.ChallengeKey, passkeyAssertionFor("person-2"))
	realmIs(t, err, SignInRefused{})
}

func TestRealmSessionReportsALockedPasskeyOwner(t *testing.T) {
	realm := newMesailRealm(t)
	realm.locked = true
	session := realm.session(t)
	challenge, _ := session.PasskeyChallenge()
	_, err := session.PasskeySignIn(challenge.ChallengeKey, `{"response":{"userHandle":"`+base64.RawURLEncoding.EncodeToString([]byte("person-1"))+`","signature":"x"}}`)
	realmIs(t, err, AccountLocked{})
}

func TestRealmSessionRefusesATamperedChallengeKeyWithoutCallingKeycloak(t *testing.T) {
	realm := newMesailRealm(t)
	session := realm.session(t)
	challenge, _ := session.PasskeyChallenge()
	_, err := session.PasskeySignIn(challenge.ChallengeKey+"x", passkeyAssertionFor("person-1"))
	realmIs(t, err, SignInRefused{})
	realmEqual(t, "grants", len(realm.grants), 0)
}

func TestRealmSessionRefusesAnExpiredChallenge(t *testing.T) {
	realm := newMesailRealm(t)
	session := realm.session(t)
	challenge, _ := session.PasskeyChallenge()
	session.Now = func() time.Time { return time.Now().Add(121 * time.Second) }
	_, err := session.PasskeySignIn(challenge.ChallengeKey, passkeyAssertionFor("person-1"))
	realmIs(t, err, SignInRefused{})
	realmEqual(t, "grants", len(realm.grants), 0)
}

func TestRealmSessionRefusesAHugeCredential(t *testing.T) {
	realm := newMesailRealm(t)
	session := realm.session(t)
	challenge, _ := session.PasskeyChallenge()
	_, err := session.PasskeySignIn(challenge.ChallengeKey, strings.Repeat("a", 64*1024+1))
	realmIs(t, err, SignInRefused{})
}
