type ThreadShare struct {
	Id           string `json:"id"`
	Subject      string `json:"subject"`
	MessageCount int    `json:"messageCount"`
	Active       bool   `json:"active"`
	RevokedAt    string `json:"revokedAt"`
	ExpiresAt    string `json:"expiresAt"`
	StatusPrefix string `json:"statusPrefix"`
	StatusDate   string `json:"statusDate"`
}
