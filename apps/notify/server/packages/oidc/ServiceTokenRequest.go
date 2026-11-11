package oidc

type ServiceTokenRequest struct {
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	DomainId     string `json:"domainId"`
}
