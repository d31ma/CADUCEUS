package keycloak

import "fmt"

type AdminRefused struct {
	Status int
	Action string
}

func (refused AdminRefused) Error() string {
	return fmt.Sprintf("Keycloak refused to %s (HTTP status %d)", refused.Action, refused.Status)
}
