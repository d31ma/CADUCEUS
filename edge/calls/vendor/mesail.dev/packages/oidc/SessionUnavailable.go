package oidc

type SessionUnavailable struct{}

func (SessionUnavailable) Error() string {
	return "sign-in is not available now"
}
