package login

import "mesail.dev/packages/oidc"

type RealmSessions struct {
	Logins  Logins
	Account oidc.RealmAccount
	Gate    oidc.Gate
}

func (sessions RealmSessions) SignIn(login string, password string, oneTimeCode string, kept bool) (Grant, error) {
	return sessions.Logins.SignIn(login, password, oneTimeCode, kept)
}

func (sessions RealmSessions) Refresh(sessionKey string) (Grant, error) {
	return sessions.Logins.Refresh(sessionKey)
}

func (sessions RealmSessions) Switch(sessionKey string, domainId string) (Grant, error) {
	return sessions.Logins.Switch(sessionKey, domainId)
}

func (sessions RealmSessions) SignOut(sessionKey string) error {
	return sessions.Logins.SignOut(sessionKey)
}

func (sessions RealmSessions) SignOutEverywhere(authorization string) error {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return err
	}
	err = sessions.Logins.EndSessions(caller.MemberId)
	if err != nil {
		return err
	}
	return sessions.Account.EndAllSessions(caller)
}

func (sessions RealmSessions) Identify(authorization string) (oidc.Caller, error) {
	return sessions.Gate.Identify(authorization)
}

func (sessions RealmSessions) ChangePassword(authorization string, change PasswordChange) (Grant, error) {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return Grant{}, err
	}
	check := oidc.PasswordCheck{Password: change.CurrentPassword, OneTimeCode: change.OneTimeCode, Kept: change.Kept}
	opened, err := sessions.Account.ChangePassword(caller, check, change.NewPassword)
	if err != nil {
		return Grant{}, err
	}
	return sessions.Logins.Restarted(opened, caller.DomainId, change.Kept)
}

func (sessions RealmSessions) TotpEnabled(authorization string) (bool, error) {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return false, err
	}
	return sessions.Account.TotpEnabled(caller)
}

func (sessions RealmSessions) SetupTotp(authorization string, current string) (oidc.TotpSetup, error) {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return oidc.TotpSetup{}, err
	}
	return sessions.Account.SetupTotp(caller, current)
}

func (sessions RealmSessions) ConfirmTotp(authorization string, setupKey string, code string) error {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return err
	}
	return sessions.Account.ConfirmTotp(caller, setupKey, code)
}

func (sessions RealmSessions) RemoveTotp(authorization string, current string, oneTimeCode string) error {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return err
	}
	return sessions.Account.RemoveTotp(caller, current, oneTimeCode)
}

func (sessions RealmSessions) PasskeyChallenge() (oidc.PasskeyChallenge, error) {
	return sessions.Logins.Realm.PasskeyChallenge()
}

func (sessions RealmSessions) PasskeySignIn(challengeKey string, credentialJson string, domainId string, kept bool) (Grant, error) {
	return sessions.Logins.PasskeySignIn(challengeKey, credentialJson, domainId, kept)
}

func (sessions RealmSessions) Passkeys(authorization string) ([]oidc.Passkey, error) {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return nil, err
	}
	return sessions.Account.Passkeys().List(caller)
}

func (sessions RealmSessions) SetupPasskey(authorization string, current string, oneTimeCode string) (oidc.PasskeySetup, error) {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return oidc.PasskeySetup{}, err
	}
	return sessions.Account.Passkeys().Setup(caller, current, oneTimeCode)
}

func (sessions RealmSessions) ConfirmPasskey(authorization string, setupKey string, credentialJson string, label string) (oidc.Passkey, error) {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return oidc.Passkey{}, err
	}
	return sessions.Account.Passkeys().Confirm(caller, setupKey, credentialJson, label)
}

func (sessions RealmSessions) RenamePasskey(authorization string, passkeyId string, label string) (oidc.Passkey, error) {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return oidc.Passkey{}, err
	}
	return sessions.Account.Passkeys().Rename(caller, passkeyId, label)
}

func (sessions RealmSessions) RemovePasskey(authorization string, passkeyId string, removal PasskeyRemoval) (Grant, error) {
	caller, err := sessions.Gate.Identify(authorization)
	if err != nil {
		return Grant{}, err
	}
	check := oidc.PasswordCheck{Password: removal.CurrentPassword, OneTimeCode: removal.OneTimeCode, Kept: removal.Kept}
	checked, err := sessions.Account.Passkeys().Remove(caller, passkeyId, check)
	if err != nil {
		return Grant{}, err
	}
	return sessions.Logins.Restarted(checked, caller.DomainId, removal.Kept)
}
