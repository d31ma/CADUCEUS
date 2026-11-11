package oidc

type KeyFetcher interface {
	Fetch(jwksUrl string) (KeySet, error)
}
