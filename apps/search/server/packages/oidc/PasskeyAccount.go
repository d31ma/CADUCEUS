package oidc

import "errors"

type PasskeyAccount struct {
	Account RealmAccount
}

func (account PasskeyAccount) List(caller Caller) ([]Passkey, error) {
	records, err := account.list(caller)
	return account.shown(records), AccountErrors{}.Normalized(err)
}

func (account PasskeyAccount) Setup(caller Caller, current string, oneTimeCode string) (PasskeySetup, error) {
	setup, err := account.setup(caller, current, oneTimeCode)
	return setup, AccountErrors{}.Normalized(err)
}

func (account PasskeyAccount) Confirm(caller Caller, setupKey string, credentialJson string, label string) (Passkey, error) {
	added, err := account.confirm(caller, setupKey, credentialJson, label)
	return added.Passkey(), AccountErrors{}.Normalized(err)
}

func (account PasskeyAccount) Rename(caller Caller, passkeyId string, label string) (Passkey, error) {
	renamed, err := account.rename(caller, passkeyId, label)
	return renamed.Passkey(), AccountErrors{}.Normalized(err)
}

func (account PasskeyAccount) Remove(caller Caller, passkeyId string, current string, oneTimeCode string) (KeycloakSession, error) {
	checked, err := account.remove(caller, passkeyId, current, oneTimeCode)
	return checked, AccountErrors{}.Normalized(err)
}

func (account PasskeyAccount) list(caller Caller) ([]PasskeyRecord, error) {
	passkeys, err := account.keycloak()
	if err != nil {
		return nil, err
	}
	return passkeys.List(caller.MemberId)
}

func (account PasskeyAccount) setup(caller Caller, current string, oneTimeCode string) (PasskeySetup, error) {
	ceremony, err := account.ceremony()
	if err != nil {
		return PasskeySetup{}, err
	}
	checked, err := account.Account.checkedPassword(caller, current, oneTimeCode)
	if err != nil {
		return PasskeySetup{}, err
	}
	_ = account.Account.Session.signOut(checked.RefreshToken)
	records, err := account.list(caller)
	if err != nil {
		return PasskeySetup{}, err
	}
	err = account.room(records)
	if err != nil {
		return PasskeySetup{}, err
	}
	return ceremony.Setup(caller, records)
}

func (account PasskeyAccount) confirm(caller Caller, setupKey string, credentialJson string, label string) (PasskeyRecord, error) {
	ceremony, err := account.ceremony()
	if err != nil {
		return PasskeyRecord{}, err
	}
	challenge, err := ceremony.SetupChallenge(caller, setupKey)
	if err != nil {
		return PasskeyRecord{}, err
	}
	passkeys, err := account.keycloak()
	if err != nil {
		return PasskeyRecord{}, err
	}
	records, err := passkeys.List(caller.MemberId)
	if err != nil {
		return PasskeyRecord{}, err
	}
	request := PasskeyAddRequest{CredentialJson: credentialJson, Challenge: challenge, Origins: ceremony.Origins.List(), Label: PasskeyLabels{}.Unique(label, records)}
	added, err := passkeys.Add(caller.MemberId, request)
	return added, account.addFailure(err)
}

func (account PasskeyAccount) rename(caller Caller, passkeyId string, label string) (PasskeyRecord, error) {
	err := PasskeyLabels{}.Checked(label)
	if err != nil {
		return PasskeyRecord{}, err
	}
	passkeys, err := account.keycloak()
	if err != nil {
		return PasskeyRecord{}, err
	}
	renamed, err := passkeys.Rename(caller.MemberId, passkeyId, label)
	return renamed, account.renameFailure(err)
}

func (account PasskeyAccount) remove(caller Caller, passkeyId string, current string, oneTimeCode string) (KeycloakSession, error) {
	passkeys, err := account.keycloak()
	if err != nil {
		return KeycloakSession{}, err
	}
	checked, err := account.Account.checkedPassword(caller, current, oneTimeCode)
	if err != nil {
		return KeycloakSession{}, err
	}
	err = passkeys.Remove(caller.MemberId, passkeyId)
	if err != nil {
		return KeycloakSession{}, account.closed(checked, err)
	}
	return checked, account.Account.manager().endOtherSessions(passkeys.Admin, caller.MemberId, checked.SessionId)
}

func (account PasskeyAccount) keycloak() (KeycloakPasskeys, error) {
	admin, err := account.Account.Session.admin()
	return KeycloakPasskeys{Admin: admin}, err
}

func (account PasskeyAccount) ceremony() (PasskeyCeremony, error) {
	return account.Account.Session.Sessions.ceremony(account.Account.Now)
}

func (account PasskeyAccount) closed(checked KeycloakSession, refused error) error {
	_ = account.Account.Session.signOut(checked.RefreshToken)
	return refused
}

func (PasskeyAccount) shown(records []PasskeyRecord) []Passkey {
	passkeys := []Passkey{}
	for _, record := range records {
		passkeys = append(passkeys, record.Passkey())
	}
	return passkeys
}

func (account PasskeyAccount) room(records []PasskeyRecord) error {
	if len(records) >= account.most() {
		return TooManyPasskeys{}
	}
	return nil
}

func (PasskeyAccount) addFailure(err error) error {
	var refused PasskeyFailure
	if errors.As(err, &refused) {
		return InvalidPasskey{}
	}
	return err
}

func (PasskeyAccount) renameFailure(err error) error {
	refused := PasskeyRefused{}
	if errors.As(err, &refused) {
		return InvalidLabel{}
	}
	return err
}

func (PasskeyAccount) most() int {
	return 10
}
