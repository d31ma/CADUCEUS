type PasskeyRequested struct {
	Mode           string   `json:"mode"`
	Challenge      string   `json:"challenge"`
	RelyingPartyID string   `json:"relyingPartyId"`
	Credentials []string `json:"credentialIds"`
	UserID         string   `json:"userId"`
	UserName       string   `json:"userName"`
}
