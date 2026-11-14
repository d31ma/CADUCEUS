package oidc

type PasskeyFailure interface {
	error
	PasskeyCode() string
}
