package oidc

type Settings struct {
	Issuer        string `json:"issuer"`
	Jwks          string `json:"jwks"`
	Audience      string `json:"audience"`
	Keycloak      string `json:"keycloak,omitempty"`
	Admin         string `json:"admin,omitempty"`
	ServiceTokens string `json:"serviceTokens,omitempty"`
}
