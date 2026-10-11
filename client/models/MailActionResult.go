type MailActionResult struct {
	ActionID  string `json:"actionId"`
	ExpiresAt string `json:"expiresAt"`
	Conflict  bool   `json:"conflict"`
}
