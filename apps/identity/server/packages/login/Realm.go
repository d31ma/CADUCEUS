package login

import "mesail.dev/packages/oidc"

type Realm interface {
	SignIn(username string, password string, oneTimeCode string, kept bool) (oidc.KeycloakSession, error)
	Refresh(refreshToken string) (oidc.KeycloakSession, error)
	SignOut(refreshToken string) error
	Seal(content oidc.SessionContent) (string, error)
	Open(sessionKey string) (oidc.SessionContent, error)
	PasskeyChallenge() (oidc.PasskeyChallenge, error)
	PasskeySignIn(challengeKey string, credentialJson string, kept bool) (oidc.KeycloakSession, error)
}
