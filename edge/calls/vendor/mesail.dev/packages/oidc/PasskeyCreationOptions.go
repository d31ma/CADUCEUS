package oidc

type PasskeyCreationOptions struct {
	Challenge              string              `json:"challenge"`
	RelyingParty           PasskeyRelyingParty `json:"rp"`
	User                   PasskeyUser         `json:"user"`
	Parameters             []PasskeyParameter  `json:"pubKeyCredParams"`
	Timeout                int64               `json:"timeout"`
	ExcludeCredentials     []PasskeyDescriptor `json:"excludeCredentials"`
	AuthenticatorSelection PasskeySelection    `json:"authenticatorSelection"`
	Attestation            string              `json:"attestation"`
}
