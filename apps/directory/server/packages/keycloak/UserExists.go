package keycloak

import "fmt"

type UserExists struct {
	Username string
}

func (exists UserExists) Error() string {
	return fmt.Sprintf("a Keycloak user named %q already exists", exists.Username)
}
