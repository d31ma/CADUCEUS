package oidc

import (
	"encoding/json"

	"slices"
	"time"
)

type AccountManager struct {
	Sessions SessionIssuer
	Admin    string
	Now      func() time.Time
}

func (manager AccountManager) TotpEnabled(authorization string) (bool, error) {
	enabled, err := manager.totpEnabled(authorization)
	return enabled, AccountErrors{}.Normalized(err)
}

func (manager AccountManager) ChangePassword(authorization string, current string, next string, oneTimeCode string) (SessionGrant, error) {
	grant, err := manager.changePassword(authorization, current, next, oneTimeCode)
	return grant, AccountErrors{}.Normalized(err)
}

func (manager AccountManager) SetupTotp(authorization string, current string) (TotpSetup, error) {
	setup, err := manager.setupTotp(authorization, current)
	return setup, AccountErrors{}.Normalized(err)
}

func (manager AccountManager) ConfirmTotp(authorization string, setupKey string, code string) error {
	return AccountErrors{}.Normalized(manager.confirmTotp(authorization, setupKey, code))
}

func (manager AccountManager) RemoveTotp(authorization string, current string, oneTimeCode string) error {
	return AccountErrors{}.Normalized(manager.removeTotp(authorization, current, oneTimeCode))
}

func (manager AccountManager) totpEnabled(authorization string) (bool, error) {
	caller, admin, err := manager.callerAndAdmin(authorization)
	if err != nil {
		return false, err
	}
	credentials, err := admin.Credentials(caller.MemberId)
	return slices.ContainsFunc(credentials, manager.isOtp), err
}

func (manager AccountManager) changePassword(authorization string, current string, next string, oneTimeCode string) (SessionGrant, error) {
	caller, admin, err := manager.callerAndAdmin(authorization)
	if err != nil {
		return SessionGrant{}, err
	}
	tokens, err := manager.checkedPassword(caller, current, oneTimeCode)
	if err != nil {
		return SessionGrant{}, err
	}
	grant, err := manager.Sessions.granted(caller.DomainId, tokens)
	if err != nil {
		return SessionGrant{}, err
	}
	err = admin.ResetPassword(caller.MemberId, next)
	if err != nil {
		return SessionGrant{}, err
	}
	return grant, manager.endOtherSessions(admin, caller.MemberId, grant.Caller.SessionId)
}

func (manager AccountManager) setupTotp(authorization string, current string) (TotpSetup, error) {
	caller, err := manager.caller(authorization)
	if err != nil {
		return TotpSetup{}, err
	}
	tokens, err := manager.checkedPassword(caller, current, "")
	if err != nil {
		return TotpSetup{}, err
	}
	_ = manager.keycloak(caller).SignOut(tokens.RefreshToken)
	secret, err := Totp{}.NewSecret()
	if err != nil {
		return TotpSetup{}, err
	}
	return manager.setup(caller, secret)
}

func (manager AccountManager) setup(caller Caller, secret string) (TotpSetup, error) {
	uri := Totp{}.Uri(caller.Email, secret)
	image, _ := QrCode{}.DataUri(uri)
	content, err := json.Marshal(TotpSetupContent{Secret: secret, Subject: caller.MemberId, Domain: caller.DomainId, ExpiresAt: manager.Now().Unix() + manager.setupSeconds()})
	if err != nil {
		return TotpSetup{}, err
	}
	setupKey, err := manager.Sessions.Sealer.Seal(string(content), manager.setupLabel())
	return TotpSetup{Secret: secret, Uri: uri, QrImage: image, SetupKey: setupKey}, err
}

func (manager AccountManager) confirmTotp(authorization string, setupKey string, code string) error {
	caller, admin, err := manager.callerAndAdmin(authorization)
	if err != nil {
		return err
	}
	content, err := manager.openedSetup(setupKey)
	if err != nil {
		return err
	}
	err = manager.validSetup(caller, content, code)
	if err != nil {
		return err
	}
	return admin.SetOtp(caller.MemberId, content.Secret)
}

func (manager AccountManager) removeTotp(authorization string, current string, oneTimeCode string) error {
	caller, admin, err := manager.callerAndAdmin(authorization)
	if err != nil {
		return err
	}
	tokens, err := manager.checkedPassword(caller, current, oneTimeCode)
	if err != nil {
		return err
	}
	_ = manager.keycloak(caller).SignOut(tokens.RefreshToken)
	credentials, err := admin.Credentials(caller.MemberId)
	if err != nil {
		return err
	}
	return manager.deleteOtp(admin, caller.MemberId, slices.DeleteFunc(credentials, manager.isNotOtp))
}

func (manager AccountManager) caller(authorization string) (Caller, error) {
	err := manager.Sessions.Problem
	if err != nil {
		return Caller{}, err
	}
	return manager.Sessions.Gate.Identify(authorization)
}

func (manager AccountManager) callerAndAdmin(authorization string) (Caller, KeycloakAdmin, error) {
	caller, err := manager.caller(authorization)
	if err != nil {
		return Caller{}, KeycloakAdmin{}, err
	}
	admin, err := manager.admin(caller.DomainId)
	return caller, admin, err
}

func (manager AccountManager) admin(domain string) (KeycloakAdmin, error) {
	keycloak, err := manager.Sessions.keycloak(domain)
	if err != nil {
		return KeycloakAdmin{}, err
	}
	keycloak.ClientId = manager.accountClientId()
	keycloak.ClientSecret = manager.Sessions.Settings.Account.Secret
	grant, err := keycloak.ClientCredentials()
	if err != nil {
		return KeycloakAdmin{}, err
	}
	endpoint, err := IssuerPattern{Pattern: manager.Admin}.Expand(domain)
	return KeycloakAdmin{Client: manager.Sessions.Client, Endpoint: endpoint, Token: grant.AccessToken}, err
}

func (manager AccountManager) keycloak(caller Caller) KeycloakClient {
	keycloak, _ := manager.Sessions.keycloak(caller.DomainId)
	return keycloak
}

func (manager AccountManager) checkedPassword(caller Caller, password string, oneTimeCode string) (TokenGrant, error) {
	return manager.keycloak(caller).SignIn(caller.Email, password, oneTimeCode)
}

func (manager AccountManager) openedSetup(setupKey string) (TotpSetupContent, error) {
	plain, err := manager.Sessions.Sealer.Open(setupKey, manager.setupLabel())
	if err != nil {
		return TotpSetupContent{}, InvalidCode{}
	}
	content := TotpSetupContent{}
	return content, json.Unmarshal([]byte(plain), &content)
}

func (manager AccountManager) validSetup(caller Caller, content TotpSetupContent, code string) error {
	if content.Subject != caller.MemberId || content.Domain != caller.DomainId || manager.Now().Unix() >= content.ExpiresAt || !(Totp{}).Matches(content.Secret, code, manager.Now()) {
		return InvalidCode{}
	}
	return nil
}

func (manager AccountManager) endOtherSessions(admin KeycloakAdmin, userId string, keep string) error {
	sessions, err := admin.Sessions(userId)
	if err != nil {
		return err
	}
	others := slices.DeleteFunc(sessions, func(session UserSession) bool { return session.Id == keep })
	return manager.endSessions(admin, others)
}

func (AccountManager) endSessions(admin KeycloakAdmin, sessions []UserSession) error {
	for _, session := range sessions {
		err := admin.EndSession(session.Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (AccountManager) deleteOtp(admin KeycloakAdmin, userId string, credentials []UserCredential) error {
	for _, credential := range credentials {
		err := admin.DeleteCredential(userId, credential.Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (manager AccountManager) isOtp(credential UserCredential) bool {
	return credential.Type == manager.otpType()
}

func (manager AccountManager) isNotOtp(credential UserCredential) bool {
	return credential.Type != manager.otpType()
}

func (AccountManager) otpType() string {
	return "otp"
}

func (AccountManager) accountClientId() string {
	return "mesail-account"
}

func (AccountManager) setupLabel() string {
	return "mesail-totp-setup"
}

func (AccountManager) setupSeconds() int64 {
	return 600
}
