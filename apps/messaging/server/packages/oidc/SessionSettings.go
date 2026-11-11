package oidc

type SessionSettings struct {
	Keycloak string        `json:"keycloak"`
	Client   ClientSetting `json:"client"`
	Session  KeySetting    `json:"session"`
}
