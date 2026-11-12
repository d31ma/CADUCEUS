package oidc

type PasskeyAddRequest struct {
	CredentialJson string   `json:"credentialJson"`
	Challenge      string   `json:"challenge"`
	Origins        []string `json:"origins"`
	Label          string   `json:"label"`
}
