package oidc

type SessionGrant struct {
	Caller      Caller
	AccessToken string
	ExpiresIn   int64
	SessionKey  string
}
