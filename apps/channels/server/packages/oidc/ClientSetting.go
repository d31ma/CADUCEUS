package oidc

type ClientSetting struct {
	Secret   string `json:"secret"`
	Previous string `json:"previous,omitempty"`
}
