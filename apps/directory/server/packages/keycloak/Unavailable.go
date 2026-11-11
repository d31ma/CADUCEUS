package keycloak

import "fmt"

type Unavailable struct {
	Status int
}

func (unavailable Unavailable) Error() string {
	return fmt.Sprintf("Keycloak is unavailable (HTTP status %d)", unavailable.Status)
}
