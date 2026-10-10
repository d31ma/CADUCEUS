type PasskeyAnswered struct {
	Mode              string `json:"mode"`
	CredentialID      string `json:"credentialId"`
	AuthenticatorData string `json:"authenticatorData"`
	ClientDataJSON    string `json:"clientDataJSON"`
	Signature         string `json:"signature"`
	AttestationObject string `json:"attestationObject"`
	Failure           string `json:"failure"`
}
