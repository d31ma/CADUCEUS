package oidc

type OtpCredentialData struct {
	SubType        string `json:"subType"`
	Digits         int    `json:"digits"`
	Period         int64  `json:"period"`
	Algorithm      string `json:"algorithm"`
	Counter        int    `json:"counter"`
	SecretEncoding string `json:"secretEncoding"`
}
