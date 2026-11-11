package oidc

type Claims struct {
	Issuer          string
	Subject         string
	Audience        []string
	AuthorizedParty string
	ExpiresAt       int64
	NotBefore       int64
	TokenType       string
	SessionId       string
	Name            string
	Email           string
	RealmRoles      []string
}
