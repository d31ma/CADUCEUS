package oidc

type KeycloakServiceToken struct {
	Keycloak KeycloakClient
}

func (token KeycloakServiceToken) CacheKey() string {
	return token.Keycloak.Endpoint
}

func (token KeycloakServiceToken) Fetch() (TokenGrant, error) {
	return token.Keycloak.ClientCredentials()
}
