package oidc

import "fmt"

type KeycloakUnavailable struct {
	Status int
}

func (unavailable KeycloakUnavailable) Error() string {
	return fmt.Sprintf("Keycloak could not answer (status %d)", unavailable.Status)
}
