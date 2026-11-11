package login

import "mesail.dev/packages/oidc"

type Sessions interface {
	SignIn(login string, password string, oneTimeCode string) (Grant, error)
	Refresh(sessionKey string) (Grant, error)
	Switch(sessionKey string, domainId string) (Grant, error)
	SignOut(sessionKey string) error
	Identify(authorization string) (oidc.Caller, error)
	ChangePassword(authorization string, current string, next string, oneTimeCode string) (Grant, error)
	TotpEnabled(authorization string) (bool, error)
	SetupTotp(authorization string, current string) (oidc.TotpSetup, error)
	ConfirmTotp(authorization string, setupKey string, code string) error
	RemoveTotp(authorization string, current string, oneTimeCode string) error
}
