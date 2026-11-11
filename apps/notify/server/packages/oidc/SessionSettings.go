package oidc

type SessionSettings struct {
	Client  ClientSetting `json:"client"`
	Session KeySetting    `json:"session"`
	Account ClientSetting `json:"account"`
}
