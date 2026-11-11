type SessionPending struct {
	DomainId string `json:"domainId"`
	Email    string `json:"email"`
	State    string `json:"state"`
	Verifier string `json:"verifier"`
}
