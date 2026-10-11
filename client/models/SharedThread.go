type SharedThread struct {
	Subject        string          `json:"subject"`
	ExpiresAt      string          `json:"expiresAt"`
	ExpiresAtLabel string          `json:"expiresAtLabel"`
	Messages       []SharedMessage `json:"messages"`
	MessageCount   string          `json:"messageCount"`
}
