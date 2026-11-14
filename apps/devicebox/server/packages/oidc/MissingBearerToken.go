package oidc

type MissingBearerToken struct{}

func (MissingBearerToken) Error() string {
	return "the Authorization header has no bearer token"
}
