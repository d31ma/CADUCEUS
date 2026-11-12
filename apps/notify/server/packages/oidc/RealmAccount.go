package oidc

import (
	"errors"
	"slices"
	"time"
)

type RealmAccount struct {
	Session RealmSession
	Now     func() time.Time
}

func (account RealmAccount) TotpEnabled(caller Caller) (bool, error) {
	enabled, err := account.totpEnabled(caller)
	return enabled, AccountErrors{}.Normalized(err)
}

func (account RealmAccount) ChangePassword(caller Caller, current string, next string, oneTimeCode string) (KeycloakSession, error) {
	checked, err := account.changePassword(caller, current, next, oneTimeCode)
	return checked, AccountErrors{}.Normalized(err)
}

func (account RealmAccount) SetupTotp(caller Caller, current string) (TotpSetup, error) {
	setup, err := account.setupTotp(caller, current)
	return setup, AccountErrors{}.Normalized(err)
}

func (account RealmAccount) ConfirmTotp(caller Caller, setupKey string, code string) error {
	return AccountErrors{}.Normalized(account.confirmTotp(caller, setupKey, code))
}

func (account RealmAccount) RemoveTotp(caller Caller, current string, oneTimeCode string) error {
	return AccountErrors{}.Normalized(account.removeTotp(caller, current, oneTimeCode))
}

func (account RealmAccount) Passkeys() PasskeyAccount {
	return PasskeyAccount{Account: account}
}

func (account RealmAccount) totpEnabled(caller Caller) (bool, error) {
	admin, err := account.Session.admin()
	if err != nil {
		return false, err
	}
	credentials, err := admin.Credentials(caller.MemberId)
	return slices.ContainsFunc(credentials, account.manager().isOtp), err
}

func (account RealmAccount) changePassword(caller Caller, current string, next string, oneTimeCode string) (KeycloakSession, error) {
	admin, err := account.Session.admin()
	if err != nil {
		return KeycloakSession{}, err
	}
	checked, err := account.checkedPassword(caller, current, oneTimeCode)
	if err != nil {
		return KeycloakSession{}, err
	}
	err = admin.ResetPassword(caller.MemberId, next)
	if err != nil {
		return KeycloakSession{}, err
	}
	err = admin.RemoveAttribute(caller.MemberId, account.mustChangePasswordAttribute())
	if err != nil {
		return KeycloakSession{}, err
	}
	return checked, account.manager().endOtherSessions(admin, caller.MemberId, checked.SessionId)
}

func (account RealmAccount) setupTotp(caller Caller, current string) (TotpSetup, error) {
	checked, err := account.checkedPassword(caller, current, "")
	if err != nil {
		return TotpSetup{}, err
	}
	_ = account.Session.signOut(checked.RefreshToken)
	secret, secretErr := Totp{}.NewSecret()
	setup, err := account.manager().setup(account.person(caller), secret)
	return setup, errors.Join(secretErr, err)
}

func (account RealmAccount) confirmTotp(caller Caller, setupKey string, code string) error {
	admin, err := account.Session.admin()
	if err != nil {
		return err
	}
	content, err := account.manager().openedSetup(setupKey)
	if err != nil {
		return err
	}
	err = account.manager().validSetup(account.person(caller), content, code)
	if err != nil {
		return err
	}
	return admin.SetOtp(caller.MemberId, content.Secret)
}

func (account RealmAccount) removeTotp(caller Caller, current string, oneTimeCode string) error {
	admin, err := account.Session.admin()
	if err != nil {
		return err
	}
	checked, err := account.checkedPassword(caller, current, oneTimeCode)
	if err != nil {
		return err
	}
	_ = account.Session.signOut(checked.RefreshToken)
	credentials, err := admin.Credentials(caller.MemberId)
	if err != nil {
		return err
	}
	return account.manager().deleteOtp(admin, caller.MemberId, slices.DeleteFunc(credentials, account.manager().isNotOtp))
}

func (account RealmAccount) checkedPassword(caller Caller, password string, oneTimeCode string) (KeycloakSession, error) {
	keycloak, err := account.Session.keycloak()
	if err != nil {
		return KeycloakSession{}, err
	}
	tokens, err := keycloak.SignIn(caller.Username, password, oneTimeCode)
	if err != nil {
		return KeycloakSession{}, err
	}
	return account.Session.opened(tokens)
}

func (account RealmAccount) manager() AccountManager {
	return AccountManager{Sessions: account.Session.Sessions, Now: account.Now}
}

func (RealmAccount) person(caller Caller) Caller {
	return Caller{MemberId: caller.MemberId, Email: caller.Username}
}

func (RealmAccount) mustChangePasswordAttribute() string {
	return "mesail.mustChangePassword"
}
