package oidc

type KeycloakError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}
