package oidc

type TokenRequest struct {
	Subject         string
	Name            string
	Email           string
	RealmRoles      []string
	Audience        string
	AuthorizedParty string
	ExpiresAt       int64
}
