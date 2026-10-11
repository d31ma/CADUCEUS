type PasskeyEnrollment struct {
	Challenge      string `json:"challenge"`
	RelyingPartyID string `json:"relyingPartyId"`
	PrincipalID    string `json:"principalId"`
}
