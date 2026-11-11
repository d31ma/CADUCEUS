package oidc

type NotConfigured struct{}

func (NotConfigured) Error() string {
	return "no OIDC token verifier is configured"
}
