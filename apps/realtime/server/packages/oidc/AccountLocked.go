package oidc

type AccountLocked struct{}

func (AccountLocked) Error() string {
	return "Keycloak has locked the account for a while"
}
