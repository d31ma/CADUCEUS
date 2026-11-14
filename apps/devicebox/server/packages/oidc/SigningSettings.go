package oidc

type SigningSettings struct {
	Key      string `json:"key"`
	Previous string `json:"previous,omitempty"`
}
