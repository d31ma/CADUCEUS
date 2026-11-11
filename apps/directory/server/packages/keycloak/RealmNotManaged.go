package keycloak

import "fmt"

type RealmNotManaged struct {
	Realm string
}

func (notManaged RealmNotManaged) Error() string {
	return fmt.Sprintf("the directory did not create the Keycloak realm %q, so it cannot delete it", notManaged.Realm)
}
