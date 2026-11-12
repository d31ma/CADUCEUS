//go:build blackbox

package blackbox

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAcId15SetupOptionsAskForAResidentKeyWithUserVerification(t *testing.T) {
	stack := NewStack(t)
	person := changedPassword(t, stack, createPerson(t, stack, fixture(t, stack, "ANA").AccessToken))
	signedIn := signIn(t, stack, person.Username, person.Password)
	options := parseCreationOptions(t, startPasskeySetup(t, stack, signedIn.AccessToken, person.Password).OptionsJson)
	userId := string(decode64(t, options.User.Id))
	selection := options.AuthenticatorSelection
	if options.Rp.Id == "" || userId != person.Id || options.User.Name != person.Username {
		t.Fatalf("want rp id, user id %s and user name %s, got %+v", person.Id, person.Username, options)
	}
	if selection.ResidentKey != "required" || selection.UserVerification != "required" {
		t.Fatalf("want resident key and user verification required, got %+v", selection)
	}
}

func TestAcId15SetupNeedsTheCurrentPassword(t *testing.T) {
	stack := NewStack(t)
	person := changedPassword(t, stack, createPerson(t, stack, fixture(t, stack, "ANA").AccessToken))
	signedIn := signIn(t, stack, person.Username, person.Password)
	passkeySetupReply(t, stack, signedIn.AccessToken, person.Password+"-Wrong").Expect(t, http.StatusUnauthorized)
}

func TestAcId15SetupNeedsASignedInPerson(t *testing.T) {
	stack := NewStack(t)
	passkeySetupReply(t, stack, "", "anything").Expect(t, http.StatusUnauthorized)
}

func TestAcId15AddedPasskeyIsListedWithoutKeyMaterial(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	reply := stack.Call(t, http.MethodGet, "/api/v1/identity/passkeys", owner.Session.AccessToken, nil).Expect(t, http.StatusOK)
	var list []passkey
	reply.Decode(t, &list)
	if len(list) != 1 || list[0].PasskeyId != owner.Passkey.PasskeyId || list[0].Label != "Mac" || list[0].CreatedAt == "" {
		t.Fatalf("want the one passkey labelled Mac, got %+v", list)
	}
	if bytes.Contains(reply.Body, []byte(owner.Authenticator.CredentialId())) {
		t.Fatalf("the list must not show the credential id: %s", reply.Body)
	}
}

func TestAcId15SetupExcludesThePersonsPasskeys(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	setup := startPasskeySetup(t, stack, owner.Session.AccessToken, owner.Person.Password)
	excluded := excludedIds(parseCreationOptions(t, setup.OptionsJson))
	if !containsText(excluded, owner.Authenticator.CredentialId()) {
		t.Fatalf("want the existing passkey excluded, got %v", excluded)
	}
}

func TestAcId15ConfirmRefusesACredentialMadeForAnotherSetup(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	first := startPasskeySetup(t, stack, owner.Session.AccessToken, owner.Person.Password)
	second := startPasskeySetup(t, stack, owner.Session.AccessToken, owner.Person.Password)
	credentialJson := NewAuthenticator(t).Create(t, first.OptionsJson)
	reply := confirmPasskeyReply(t, stack, owner.Session.AccessToken, second.SetupKey, credentialJson, "Phone")
	reply.ExpectError(t, http.StatusBadRequest, "invalid_passkey")
}

func TestAcId15ConfirmRefusesAnotherPersonsSetup(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	other := newPasskeyOwner(t, stack)
	setup := startPasskeySetup(t, stack, owner.Session.AccessToken, owner.Person.Password)
	credentialJson := NewAuthenticator(t).Create(t, setup.OptionsJson)
	reply := confirmPasskeyReply(t, stack, other.Session.AccessToken, setup.SetupKey, credentialJson, "Phone")
	reply.ExpectError(t, http.StatusBadRequest, "invalid_passkey")
}

func TestAcId15TenPasskeysAtMost(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	addMorePasskeys(t, stack, owner, 9)
	passkeySetupReply(t, stack, owner.Session.AccessToken, owner.Person.Password).ExpectError(t, http.StatusConflict, "too_many")
}

func TestAcId16RequestOptionsAskForADiscoverableCredential(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	options := parseRequestOptions(t, passkeyChallengeFor(t, stack).OptionsJson)
	setup := parseCreationOptions(t, startPasskeySetup(t, stack, owner.Session.AccessToken, owner.Person.Password).OptionsJson)
	if options.RpId != setup.Rp.Id || options.UserVerification != "required" || len(options.AllowCredentials) != 0 {
		t.Fatalf("want rp id %s, user verification required, no allow list, got %+v", setup.Rp.Id, options)
	}
}

func TestAcId16PasskeySignInLandsInTheDefaultDomainWithoutACode(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	signedIn := signInWithPasskey(t, stack, owner.Authenticator)
	claims := tokenClaims(t, signedIn.AccessToken)
	if claims["domain"] != "example.com" || signedIn.Me.Username != owner.Person.Username || signedIn.SessionKey == "" {
		t.Fatalf("want %s in example.com with a session key, got %+v and %v", owner.Person.Username, signedIn.Me, claims["domain"])
	}
}

func TestAcId16PasskeySignInRefusesADomainThePersonIsNotIn(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	challenge := passkeyChallengeFor(t, stack)
	request := passkeySignInRequest(challenge, owner.Authenticator.Assert(t, challenge.OptionsJson))
	request["domainId"] = "example.org"
	passkeySignInReply(t, stack, request).ExpectError(t, http.StatusForbidden, "not_a_member")
}

func TestAcId16PasskeySignInMayKeepTheDevice(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	challenge := passkeyChallengeFor(t, stack)
	request := passkeySignInRequest(challenge, owner.Authenticator.Assert(t, challenge.OptionsJson))
	request["keepSignedIn"] = true
	var signedIn session
	passkeySignInReply(t, stack, request).Expect(t, http.StatusOK).Decode(t, &signedIn)
	if !signedIn.Kept {
		t.Fatalf("want a kept session, got %+v", signedIn)
	}
}

func TestAcId16WrongSignatureGetsTheSameAnswerAsAWrongPassword(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	challenge := passkeyChallengeFor(t, stack)
	request := passkeySignInRequest(challenge, owner.Authenticator.AssertWrongSignature(t, challenge.OptionsJson))
	passkeyFailure := passkeySignInReply(t, stack, request).Expect(t, http.StatusUnauthorized).Error(t)
	stranger := changedPassword(t, stack, createPerson(t, stack, fixture(t, stack, "ANA").AccessToken))
	passwordFailure := signInReply(t, stack, stranger.Username, stranger.Password+"-Wrong").Expect(t, http.StatusUnauthorized).Error(t)
	if passkeyFailure != passwordFailure {
		t.Fatalf("want the same answer as a wrong password %+v, got %+v", passwordFailure, passkeyFailure)
	}
}

func TestAcId16ReplayedChallengeIsRefused(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	challenge := passkeyChallengeFor(t, stack)
	request := passkeySignInRequest(challenge, owner.Authenticator.Assert(t, challenge.OptionsJson))
	passkeySignInReply(t, stack, request).Expect(t, http.StatusOK)
	passkeySignInReply(t, stack, request).ExpectError(t, http.StatusUnauthorized, "unauthenticated")
}

func TestAcId16AssertionForAnotherChallengeIsRefused(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	signed := passkeyChallengeFor(t, stack)
	sent := passkeyChallengeFor(t, stack)
	request := passkeySignInRequest(sent, owner.Authenticator.Assert(t, signed.OptionsJson))
	passkeySignInReply(t, stack, request).ExpectError(t, http.StatusUnauthorized, "unauthenticated")
}

func TestAcId16TamperedChallengeKeyIsRefused(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	challenge := passkeyChallengeFor(t, stack)
	request := passkeySignInRequest(challenge, owner.Authenticator.Assert(t, challenge.OptionsJson))
	request["challengeKey"] = challenge.ChallengeKey + "x"
	passkeySignInReply(t, stack, request).ExpectError(t, http.StatusUnauthorized, "unauthenticated")
}

func TestAcId16ExpiredChallengeIsRefused(t *testing.T) {
	stack := NewStack(t)
	if os.Getenv("MESAIL_PASSKEY_EXPIRY_TEST") == "" {
		t.Skip("MESAIL_PASSKEY_EXPIRY_TEST is not set; this test waits 125 seconds")
	}
	owner := newPasskeyOwner(t, stack)
	challenge := passkeyChallengeFor(t, stack)
	request := passkeySignInRequest(challenge, owner.Authenticator.Assert(t, challenge.OptionsJson))
	time.Sleep(125 * time.Second)
	passkeySignInReply(t, stack, request).ExpectError(t, http.StatusUnauthorized, "unauthenticated")
}

func TestAcId16UnknownPasskeyIsRefused(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	challenge := passkeyChallengeFor(t, stack)
	stranger := NewAuthenticator(t)
	request := passkeySignInRequest(challenge, stranger.AssertAs(t, challenge.OptionsJson, owner.Person.Id))
	passkeySignInReply(t, stack, request).ExpectError(t, http.StatusUnauthorized, "unauthenticated")
}

func TestAcId16PasskeyCannotSignInAsAnotherPerson(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	other := newPasskeyOwner(t, stack)
	challenge := passkeyChallengeFor(t, stack)
	request := passkeySignInRequest(challenge, owner.Authenticator.AssertAs(t, challenge.OptionsJson, other.Person.Id))
	passkeySignInReply(t, stack, request).ExpectError(t, http.StatusUnauthorized, "unauthenticated")
}

func TestAcId16PasskeySignInIsShownAsLastUsed(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	signedIn := signInWithPasskey(t, stack, owner.Authenticator)
	list := listPasskeys(t, stack, signedIn.AccessToken)
	if len(list) != 1 || list[0].LastUsedAt == "" {
		t.Fatalf("want lastUsedAt after a passkey sign-in, got %+v", list)
	}
}

func TestAcId17RemovingAPasskeyNeedsTheCurrentPassword(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	removePasskeyReply(t, stack, owner.Session.AccessToken, owner.Passkey.PasskeyId, owner.Person.Password+"-Wrong").Expect(t, http.StatusUnauthorized)
	signInWithPasskey(t, stack, owner.Authenticator)
}

func TestAcId17RemovingAPasskeyEndsTheOtherSessions(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	phone := keptSignIn(t, stack, owner.Person.Username, owner.Person.Password, true)
	var fresh session
	removePasskeyReply(t, stack, owner.Session.AccessToken, owner.Passkey.PasskeyId, owner.Person.Password).Expect(t, http.StatusOK).Decode(t, &fresh)
	refreshReply(t, stack, phone.SessionKey).ExpectError(t, http.StatusForbidden, "access_revoked")
	refreshReply(t, stack, fresh.SessionKey).Expect(t, http.StatusOK)
	if ids := passkeyIds(listPasskeys(t, stack, fresh.AccessToken)); len(ids) != 0 {
		t.Fatalf("want no passkeys left, got %v", ids)
	}
}

func TestAcId17RemovedPasskeyCannotSignIn(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	removePasskeyReply(t, stack, owner.Session.AccessToken, owner.Passkey.PasskeyId, owner.Person.Password).Expect(t, http.StatusOK)
	challenge := passkeyChallengeFor(t, stack)
	request := passkeySignInRequest(challenge, owner.Authenticator.Assert(t, challenge.OptionsJson))
	passkeySignInReply(t, stack, request).ExpectError(t, http.StatusUnauthorized, "unauthenticated")
}

func TestAcId17AnotherPersonsPasskeyIsNotFound(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	other := newPasskeyOwner(t, stack)
	reply := removePasskeyReply(t, stack, other.Session.AccessToken, owner.Passkey.PasskeyId, other.Person.Password)
	reply.ExpectError(t, http.StatusNotFound, "not_found")
	signInWithPasskey(t, stack, owner.Authenticator)
}

func TestAcId18PasswordSignInKeepsWorkingWithAPasskey(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	signIn(t, stack, owner.Person.Username, owner.Person.Password)
}

func TestAcId13AdminRemovesAllPasskeysAndEndsTheSessions(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	addMorePasskeys(t, stack, owner, 1)
	phone := keptSignIn(t, stack, owner.Person.Username, owner.Person.Password, true)
	path := "/api/v1/directory/people/" + owner.Person.Id + "/passkeys"
	stack.Call(t, http.MethodDelete, path, fixture(t, stack, "ANA").AccessToken, nil).Expect(t, http.StatusNoContent)
	refreshReply(t, stack, phone.SessionKey).ExpectError(t, http.StatusForbidden, "access_revoked")
	again := signIn(t, stack, owner.Person.Username, owner.Person.Password)
	if ids := passkeyIds(listPasskeys(t, stack, again.AccessToken)); len(ids) != 0 {
		t.Fatalf("want no passkeys left, got %v", ids)
	}
}

func TestAcId13PlainMemberCannotRemoveAnotherPersonsPasskeys(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	path := "/api/v1/directory/people/" + owner.Person.Id + "/passkeys"
	stack.Call(t, http.MethodDelete, path, fixture(t, stack, "BO").AccessToken, nil).ExpectError(t, http.StatusForbidden, "forbidden")
	signInWithPasskey(t, stack, owner.Authenticator)
}

func TestAcId15RenamingAPasskeyNeedsNoPassword(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	var renamed passkey
	renamePasskeyReply(t, stack, owner.Session.AccessToken, owner.Passkey.PasskeyId, "Work laptop").Expect(t, http.StatusOK).Decode(t, &renamed)
	list := listPasskeys(t, stack, owner.Session.AccessToken)
	if renamed.Label != "Work laptop" || len(list) != 1 || list[0].Label != "Work laptop" {
		t.Fatalf("want the label Work laptop, got %+v and %+v", renamed, list)
	}
}

func TestAcId15PasskeyLabelIsOneTo64Characters(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	tooLong := strings.Repeat("a", 65)
	reply := renamePasskeyReply(t, stack, owner.Session.AccessToken, owner.Passkey.PasskeyId, tooLong)
	reply.ExpectError(t, http.StatusBadRequest, "invalid_request")
	renamePasskeyReply(t, stack, owner.Session.AccessToken, owner.Passkey.PasskeyId, "").ExpectError(t, http.StatusBadRequest, "invalid_request")
}

func TestAcId15AnotherPersonsPasskeyCannotBeRenamed(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	other := newPasskeyOwner(t, stack)
	reply := renamePasskeyReply(t, stack, other.Session.AccessToken, owner.Passkey.PasskeyId, "Mine now")
	reply.ExpectError(t, http.StatusNotFound, "not_found")
}

func TestAcId15ASecondPasskeyWithTheSameLabelGetsANumber(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	_, second := addPasskey(t, stack, owner.Session.AccessToken, owner.Person.Password, "Mac")
	if second.Label != "Mac 2" {
		t.Fatalf("want the label Mac 2, got %+v", second)
	}
}

func TestAcId15RenamingToATakenLabelIsRefused(t *testing.T) {
	stack := NewStack(t)
	owner := newPasskeyOwner(t, stack)
	_, second := addPasskey(t, stack, owner.Session.AccessToken, owner.Person.Password, "Phone")
	reply := renamePasskeyReply(t, stack, owner.Session.AccessToken, second.PasskeyId, "Mac")
	reply.ExpectError(t, http.StatusConflict, "label_exists")
}
