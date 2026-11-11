package oidc

type KeySource interface {
	Key(jwksUrl string, keyId string) (JsonWebKey, error)
}
