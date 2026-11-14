package oidc

type TotpSetupContent struct {
	Secret    string `json:"secret"`
	Subject   string `json:"subject"`
	Domain    string `json:"domain"`
	ExpiresAt int64  `json:"expiresAt"`
}
