// ThreadShare is one secret link of a conversation, with the labels the
// sharing dialog shows.
type ThreadShare struct {
	Id           string `json:"id"`
	Subject      string `json:"subject"`
	MessageCount int    `json:"messageCount"`
	Active       bool   `json:"active"`
	RevokedAt    string `json:"revokedAt"`
	ExpiresAt    string `json:"expiresAt"`
	MessageLabel string `json:"messageLabel"`
	StatusLabel  string `json:"statusLabel"`
}
