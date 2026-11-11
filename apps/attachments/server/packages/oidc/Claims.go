package oidc

type Claims struct {
	Issuer          string
	Subject         string
	Audience        []string
	AuthorizedParty string
	ExpiresAt       int64
	Name            string
	Email           string
	RealmRoles      []string
}
