package oidc

type OtpCredential struct {
	Secret    string `json:"secret"`
	Digits    int    `json:"digits"`
	Period    int64  `json:"period"`
	Algorithm string `json:"algorithm"`
}
