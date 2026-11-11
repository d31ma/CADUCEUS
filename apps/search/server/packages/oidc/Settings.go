package oidc

type Settings struct {
	Issuer   string `json:"issuer"`
	Jwks     string `json:"jwks"`
	Audience string `json:"audience"`
}
