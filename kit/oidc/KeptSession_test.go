package oidc

import (
	"net/http"
	"testing"
)

func TestSignInScopeAsksForAnOfflineSessionOnlyWhenKept(t *testing.T) {
	realmEqual(t, "kept", SignInScope{}.For(true), "openid offline_access")
	realmEqual(t, "not kept", SignInScope{}.For(false), "openid")
}

func TestKeycloakClientAsksForTheOfflineScopeOnAKeptSignIn(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusOK, `{"access_token":"a","refresh_token":"r","expires_in":60}`)
	_, err := fake.client().SignIn("ana", "pw", "", true)
	realmIs(t, err, nil)
	realmEqual(t, "scope", fake.forms[0].Get("scope"), "openid offline_access")
}

func TestKeycloakClientAsksForTheOfflineScopeOnAKeptPasskeySignIn(t *testing.T) {
	fake := newKeycloakFake(t, http.StatusOK, `{"access_token":"a","refresh_token":"r","expires_in":60}`)
	_, err := fake.client().PasskeySignIn("{}", "challenge", "https://mail.mesail.test", true)
	realmIs(t, err, nil)
	realmEqual(t, "scope", fake.forms[0].Get("scope"), "openid offline_access")
}

func TestRealmSessionSignsInForAKeptDevice(t *testing.T) {
	realm := newMesailRealm(t)
	_, err := realm.session(t).SignIn("ana", "right-password", "", true)
	realmIs(t, err, nil)
	realmList(t, "scopes", realm.scopes, []string{"openid offline_access"})
}

func TestRealmSessionSealsTheKeptFlagAndTheSignInTime(t *testing.T) {
	realm := newMesailRealm(t)
	session := realm.session(t)
	content := SessionContent{Domain: "example.com", RefreshToken: "refresh-1", PersonId: "person-1", Kept: true, SignedInAt: "2026-11-14T20:00:00.123456789Z"}
	sealed, sealErr := session.Seal(content)
	opened, openErr := session.Open(sealed)
	realmIs(t, sealErr, nil)
	realmIs(t, openErr, nil)
	realmEqual(t, "content", opened, content)
}

func TestRealmAccountKeepsTheDeviceAfterAPasswordChange(t *testing.T) {
	realm := newMesailRealm(t)
	_, err := realm.account(t).ChangePassword(realmCaller(), PasswordCheck{Password: "right-password", Kept: true}, "a-new-long-password")
	realmIs(t, err, nil)
	realmList(t, "scopes", realm.scopes, []string{"openid offline_access"})
}

func TestRealmAccountKeepsTheDeviceAfterAPasskeyRemoval(t *testing.T) {
	realm := newMesailRealm(t)
	realm.passkeys = []PasskeyRecord{passkeyRecord("passkey-1", "Mac")}
	_, err := realm.account(t).Passkeys().Remove(realmCaller(), "passkey-1", PasswordCheck{Password: "right-password", Kept: true})
	realmIs(t, err, nil)
	realmList(t, "scopes", realm.scopes, []string{"openid offline_access"})
}

func TestRealmAccountEndsEverySessionAndTheOfflineSessions(t *testing.T) {
	realm := newMesailRealm(t)
	realm.consents = true
	err := realm.account(t).EndAllSessions(realmCaller())
	realmIs(t, err, nil)
	realmEqual(t, "logged out", realm.loggedOut, 1)
	realmList(t, "revoked", realm.revoked, []string{"mesail-identity"})
}

func TestRealmAccountEndsEverySessionWhenNoOfflineSessionIsLeft(t *testing.T) {
	realm := newMesailRealm(t)
	err := realm.account(t).EndAllSessions(realmCaller())
	realmIs(t, err, nil)
	realmList(t, "revoked", realm.revoked, []string{"mesail-identity"})
}

func TestRealmAccountReportsAKeycloakFailureWhileEndingTheSessions(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t)
	realm.server.Close()
	err := account.EndAllSessions(realmCaller())
	realmIs(t, err, SessionUnavailable{})
}

func TestRealmAccountNeedsItsSettingsToEndTheSessions(t *testing.T) {
	err := RealmAccountFactory{}.FromJson("{}").EndAllSessions(realmCaller())
	realmIs(t, err, SessionUnavailable{})
}
