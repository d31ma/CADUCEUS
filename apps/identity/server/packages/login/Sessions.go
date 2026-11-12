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
	PasskeyChallenge() (oidc.PasskeyChallenge, error)
	PasskeySignIn(challengeKey string, credentialJson string, domainId string) (Grant, error)
	Passkeys(authorization string) ([]oidc.Passkey, error)
	SetupPasskey(authorization string, current string, oneTimeCode string) (oidc.PasskeySetup, error)
	ConfirmPasskey(authorization string, setupKey string, credentialJson string, label string) (oidc.Passkey, error)
	RenamePasskey(authorization string, passkeyId string, label string) (oidc.Passkey, error)
	RemovePasskey(authorization string, passkeyId string, removal PasskeyRemoval) (Grant, error)
}
