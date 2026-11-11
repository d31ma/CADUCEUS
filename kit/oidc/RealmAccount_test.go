package oidc

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func (realm *mesailRealm) account(t *testing.T) RealmAccount {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	account := RealmAccountFactory{}.FromJson(realm.document())
	account.Session.Sessions.Client = realm.server.Client()
	return account
}

func realmCaller() Caller {
	return Caller{MemberId: "person-1", Username: "ana", DomainId: "example.com"}
}

func (realm *mesailRealm) withTotp(t *testing.T, account RealmAccount) TotpSetup {
	t.Helper()
	setup, err := account.SetupTotp(realmCaller(), "right-password")
	realmIs(t, err, nil)
	code, err := Totp{}.Code(setup.Secret, time.Now())
	realmIs(t, err, nil)
	realmIs(t, account.ConfirmTotp(realmCaller(), setup.SetupKey, code), nil)
	return setup
}

func (realm *mesailRealm) currentCode(t *testing.T) string {
	t.Helper()
	code, err := Totp{}.Code(realm.otpSecret, time.Now())
	realmIs(t, err, nil)
	return code
}

func (realm *mesailRealm) storedUser(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(realm.user)
	realmIs(t, err, nil)
	return string(raw)
}

func realmAccountUnavailable(t *testing.T, account RealmAccount) {
	t.Helper()
	caller := realmCaller()
	_, enabledErr := account.TotpEnabled(caller)
	_, changeErr := account.ChangePassword(caller, "right-password", "a-new-long-password", "")
	_, setupErr := account.SetupTotp(caller, "right-password")
	confirmErr := account.ConfirmTotp(caller, "key", "123456")
	removeErr := account.RemoveTotp(caller, "right-password", "")
	for _, err := range []error{enabledErr, changeErr, setupErr, confirmErr, removeErr} {
		realmIs(t, err, SessionUnavailable{})
	}
}

func TestRealmAccountTotpIsOffWithoutAnOtpCredential(t *testing.T) {
	realm := newMesailRealm(t)
	enabled, err := realm.account(t).TotpEnabled(realmCaller())
	realmIs(t, err, nil)
	realmEqual(t, "enabled", enabled, false)
}

func TestRealmAccountTotpIsOnWithAnOtpCredential(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t)
	setup := realm.withTotp(t, account)
	enabled, err := account.TotpEnabled(realmCaller())
	realmIs(t, err, nil)
	realmEqual(t, "enabled", enabled, true)
	realmEqual(t, "stored secret", realm.otpSecret, setup.Secret)
}

func TestRealmAccountChangesThePassword(t *testing.T) {
	realm := newMesailRealm(t)
	checked, err := realm.account(t).ChangePassword(realmCaller(), "right-password", "a-new-long-password", "")
	realmIs(t, err, nil)
	realmEqual(t, "session", checked, KeycloakSession{PersonId: "person-1", SessionId: "session-a", Username: "ana", DisplayName: "Ana Example", RefreshToken: "refresh-1"})
	realmEqual(t, "password", realm.password, "a-new-long-password")
	realmList(t, "usernames", realm.usernames, []string{"ana"})
	realmList(t, "ended sessions", realm.ended, []string{"session-b"})
	realmEqual(t, "user", realm.storedUser(t), `{"attributes":{"locale":["en"]},"email":"ana@example.com","enabled":true,"id":"person-1","username":"ana"}`)
}

func TestRealmAccountRefusesAWrongCurrentPassword(t *testing.T) {
	realm := newMesailRealm(t)
	_, err := realm.account(t).ChangePassword(realmCaller(), "wrong", "a-new-long-password", "")
	realmIs(t, err, SignInRefused{})
	realmEqual(t, "password", realm.password, "right-password")
	realmList(t, "ended sessions", realm.ended, nil)
	realmEqual(t, "user", realm.storedUser(t), `{"attributes":{"locale":["en"],"mesail.mustChangePassword":["true"]},"email":"ana@example.com","enabled":true,"id":"person-1","username":"ana"}`)
}

func TestRealmAccountNamesTheFailedPasswordPolicy(t *testing.T) {
	realm := newMesailRealm(t)
	_, err := realm.account(t).ChangePassword(realmCaller(), "right-password", "short", "")
	realmIs(t, err, AdminRefused{Message: "Invalid password: minimum length 12."})
	realmEqual(t, "password", realm.password, "right-password")
	realmList(t, "ended sessions", realm.ended, nil)
}

func TestRealmAccountReportsAFailedRemoveAttribute(t *testing.T) {
	realm := newMesailRealm(t)
	caller := Caller{MemberId: "person-2", Username: "ana", DomainId: "example.com"}
	_, err := realm.account(t).ChangePassword(caller, "right-password", "a-new-long-password", "")
	realmIs(t, err, SignInRefused{})
	realmList(t, "ended sessions", realm.ended, nil)
}

func TestRemoveAttributeKeepsAUserWithoutAttributes(t *testing.T) {
	realm := newMesailRealm(t)
	realm.user = map[string]any{"id": "person-1", "username": "ana"}
	admin := KeycloakAdmin{Client: realm.server.Client(), Endpoint: realm.admin(), Token: "admin-token"}
	err := admin.RemoveAttribute("person-1", "mesail.mustChangePassword")
	realmIs(t, err, nil)
	realmEqual(t, "user", realm.storedUser(t), `{"attributes":{},"id":"person-1","username":"ana"}`)
}

func TestRemoveAttributeReportsAFailedRead(t *testing.T) {
	realm := newMesailRealm(t)
	admin := KeycloakAdmin{Client: realm.server.Client(), Endpoint: realm.admin(), Token: "wrong-token"}
	err := admin.RemoveAttribute("person-1", "mesail.mustChangePassword")
	realmIs(t, err, KeycloakUnavailable{Status: 401})
}

func TestRealmAccountSetsUpAndConfirmsTotp(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t)
	setup, err := account.SetupTotp(realmCaller(), "right-password")
	realmIs(t, err, nil)
	realmEqual(t, "sign-outs", realm.signOuts, 1)
	realmEqual(t, "uri", strings.HasPrefix(setup.Uri, "otpauth://totp/Mesail:ana?"), true)
	code, _ := Totp{}.Code(setup.Secret, time.Now())
	realmIs(t, account.ConfirmTotp(realmCaller(), setup.SetupKey, code), nil)
	realmEqual(t, "stored secret", realm.otpSecret, setup.Secret)
}

func TestRealmAccountSetupRefusesAWrongPassword(t *testing.T) {
	realm := newMesailRealm(t)
	_, err := realm.account(t).SetupTotp(realmCaller(), "wrong")
	realmIs(t, err, SignInRefused{})
	realmEqual(t, "sign-outs", realm.signOuts, 0)
}

func TestRealmAccountConfirmRefusesAnotherPerson(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t)
	setup, _ := account.SetupTotp(realmCaller(), "right-password")
	code, _ := Totp{}.Code(setup.Secret, time.Now())
	err := account.ConfirmTotp(Caller{MemberId: "person-2", Username: "bo", DomainId: "example.com"}, setup.SetupKey, code)
	realmIs(t, err, InvalidCode{})
	realmEqual(t, "stored secret", realm.otpSecret, "")
}

func TestRealmAccountConfirmRefusesAWrongCode(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t)
	setup, _ := account.SetupTotp(realmCaller(), "right-password")
	err := account.ConfirmTotp(realmCaller(), setup.SetupKey, "not-a-code")
	realmIs(t, err, InvalidCode{})
	realmEqual(t, "stored secret", realm.otpSecret, "")
}

func TestRealmAccountConfirmRefusesAForgedSetupKey(t *testing.T) {
	realm := newMesailRealm(t)
	err := realm.account(t).ConfirmTotp(realmCaller(), "forged", "123456")
	realmIs(t, err, InvalidCode{})
}

func TestRealmAccountRemovesOnlyTheOtpCredential(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t)
	realm.withTotp(t, account)
	err := account.RemoveTotp(realmCaller(), "right-password", realm.currentCode(t))
	realmIs(t, err, nil)
	realmList(t, "deleted", realm.deleted, []string{"otp-1"})
	realmEqual(t, "stored secret", realm.otpSecret, "")
	realmEqual(t, "sign-outs", realm.signOuts, 2)
}

func TestRealmAccountRemoveRefusesAWrongCode(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t)
	setup := realm.withTotp(t, account)
	err := account.RemoveTotp(realmCaller(), "right-password", "not-a-code")
	realmIs(t, err, SignInRefused{})
	realmList(t, "deleted", realm.deleted, nil)
	realmEqual(t, "stored secret", realm.otpSecret, setup.Secret)
}

func TestRealmAccountRemoveReportsFailedCredentials(t *testing.T) {
	realm := newMesailRealm(t)
	caller := Caller{MemberId: "person-2", Username: "ana", DomainId: "example.com"}
	err := realm.account(t).RemoveTotp(caller, "right-password", "")
	realmIs(t, err, SignInRefused{})
	realmList(t, "deleted", realm.deleted, nil)
}

func TestRealmAccountNeedsTheAdminEndpoint(t *testing.T) {
	realm := newMesailRealm(t)
	t.Setenv("TMPDIR", t.TempDir())
	account := RealmAccountFactory{}.FromJson(mesailDocument(realm.keycloak(), "", mesailSessionKey(), "account-secret"))
	realmAccountUnavailable(t, account)
	realmList(t, "usernames", realm.usernames, nil)
}

func TestRealmAccountNeedsTheAccountSecret(t *testing.T) {
	realm := newMesailRealm(t)
	t.Setenv("TMPDIR", t.TempDir())
	account := RealmAccountFactory{}.FromJson(mesailDocument(realm.keycloak(), realm.admin(), mesailSessionKey(), ""))
	realmAccountUnavailable(t, account)
	realmList(t, "usernames", realm.usernames, nil)
}

func TestRealmAccountFactoryReadsTheEnvironment(t *testing.T) {
	realm := newMesailRealm(t)
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("YON_APPLICATION_JSON", realm.document())
	account := RealmAccountFactory{}.Configured()
	account.Session.Sessions.Client = realm.server.Client()
	enabled, err := account.TotpEnabled(realmCaller())
	realmIs(t, err, nil)
	realmEqual(t, "enabled", enabled, false)
}
