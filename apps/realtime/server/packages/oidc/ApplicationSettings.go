package oidc

type ApplicationSettings struct {
	Oidc     Settings        `json:"oidc"`
	Identity SessionSettings `json:"identity"`
	Service  ServiceSettings `json:"service"`
}
