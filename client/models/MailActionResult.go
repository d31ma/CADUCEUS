type MailActionResult struct {
	ActionID  string        `json:"actionId"`
	ExpiresAt string        `json:"expiresAt"`
	Emails    []EmailChange `json:"emails"`
}
