package keycloak

type UserMissing struct{}

func (UserMissing) Error() string {
	return "the Keycloak user does not exist"
}
