package oidc

type CredentialRepresentation struct {
	Type           string `json:"type"`
	UserLabel      string `json:"userLabel"`
	SecretData     string `json:"secretData"`
	CredentialData string `json:"credentialData"`
}
