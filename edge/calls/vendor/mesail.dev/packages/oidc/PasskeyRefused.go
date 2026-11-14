package oidc

type PasskeyRefused struct {
	Status int
	Code   string
}

func (refused PasskeyRefused) Error() string {
	return "Keycloak refused the passkey call: " + refused.Code
}

func (refused PasskeyRefused) PasskeyCode() string {
	return refused.Code
}
