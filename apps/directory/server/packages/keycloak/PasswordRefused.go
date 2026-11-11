package keycloak

import "fmt"

type PasswordRefused struct {
	Message string
}

func (refused PasswordRefused) Error() string {
	return fmt.Sprintf("Keycloak refused the password: %s", refused.Message)
}
