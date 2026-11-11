package oidc

type AdminRefused struct {
	Message string
}

func (refused AdminRefused) Error() string {
	return "Keycloak refused the change: " + refused.Message
}
