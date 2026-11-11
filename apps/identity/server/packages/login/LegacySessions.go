package login

import (
	"slices"

	"mesail.dev/packages/oidc"
)

type LegacySessions struct {
	Issuer   oidc.SessionIssuer
	Accounts oidc.AccountManager
}

func (sessions LegacySessions) SignIn(login string, password string, oneTimeCode string) (Grant, error) {
	grant, err := sessions.Issuer.SignIn(login, password, oneTimeCode)
	return sessions.granted(grant), err
}

func (sessions LegacySessions) Refresh(sessionKey string) (Grant, error) {
	grant, err := sessions.Issuer.Refresh(sessionKey)
	return sessions.granted(grant), err
}

func (LegacySessions) Switch(sessionKey string, domainId string) (Grant, error) {
	return Grant{}, NotFound{}
}

func (sessions LegacySessions) SignOut(sessionKey string) error {
	return sessions.Issuer.SignOut(sessionKey)
}

func (sessions LegacySessions) Identify(authorization string) (oidc.Caller, error) {
	return sessions.Issuer.Gate.Identify(authorization)
}

func (sessions LegacySessions) ChangePassword(authorization string, current string, next string, oneTimeCode string) (Grant, error) {
	grant, err := sessions.Accounts.ChangePassword(authorization, current, next, oneTimeCode)
	return sessions.granted(grant), err
}

func (sessions LegacySessions) TotpEnabled(authorization string) (bool, error) {
	return sessions.Accounts.TotpEnabled(authorization)
}

func (sessions LegacySessions) SetupTotp(authorization string, current string) (oidc.TotpSetup, error) {
	return sessions.Accounts.SetupTotp(authorization, current)
}

func (sessions LegacySessions) ConfirmTotp(authorization string, setupKey string, code string) error {
	return sessions.Accounts.ConfirmTotp(authorization, setupKey, code)
}

func (sessions LegacySessions) RemoveTotp(authorization string, current string, oneTimeCode string) error {
	return sessions.Accounts.RemoveTotp(authorization, current, oneTimeCode)
}

func (LegacySessions) granted(grant oidc.SessionGrant) Grant {
	caller := grant.Caller
	admin := slices.Contains(caller.Roles, LegacySessions{}.adminRole())
	return Grant{
		PersonId:       caller.MemberId,
		DomainId:       caller.DomainId,
		Username:       caller.Username,
		DisplayName:    caller.DisplayName,
		PrimaryAddress: caller.Email,
		Roles:          caller.Roles,
		Domains:        []DomainChoice{{DomainId: caller.DomainId, Admin: admin, Default: true}},
		AccessToken:    grant.AccessToken,
		ExpiresIn:      grant.ExpiresIn,
		SessionKey:     grant.SessionKey,
	}
}

func (LegacySessions) adminRole() string {
	return "DomainAdmin"
}
