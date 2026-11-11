package oidc

type SignInRefused struct{}

func (SignInRefused) Error() string {
	return "Keycloak refused the sign-in"
}
