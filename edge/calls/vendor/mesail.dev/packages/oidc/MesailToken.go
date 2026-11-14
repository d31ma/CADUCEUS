package oidc

type MesailToken struct {
	Subject         string
	AuthorizedParty string
	Domain          string
	RealmRoles      []string
	Name            string
	Username        string
	Email           string
	SessionId       string
}
