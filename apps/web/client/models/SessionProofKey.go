type SessionProofKey struct {
	Verifier  string `json:"verifier"`
	Challenge string `json:"challenge"`
	State     string `json:"state"`
}
