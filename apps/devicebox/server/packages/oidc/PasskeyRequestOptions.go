package oidc

type PasskeyRequestOptions struct {
	Challenge        string              `json:"challenge"`
	RelyingPartyId   string              `json:"rpId"`
	Timeout          int64               `json:"timeout"`
	UserVerification string              `json:"userVerification"`
	AllowCredentials []PasskeyDescriptor `json:"allowCredentials"`
}
