package keycloak

import "fmt"

type RealmExists struct {
	Realm string
}

func (exists RealmExists) Error() string {
	return fmt.Sprintf("the Keycloak realm %q already exists", exists.Realm)
}
