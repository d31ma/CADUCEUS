type PasskeyCeremonyRequested struct {
	Challenge      string `json:"challenge"`
	RelyingPartyID string `json:"relyingPartyId"`
	UserID         string `json:"userId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
}
