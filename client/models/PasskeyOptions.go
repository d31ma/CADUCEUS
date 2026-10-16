type PasskeyOptions struct {
	Challenge      string   `json:"challenge"`
	RelyingPartyID string   `json:"relyingPartyId"`
	Credentials []string `json:"credentialIds"`
}
