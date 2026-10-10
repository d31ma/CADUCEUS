type MailActionResult struct {
	ActionId  string          `json:"actionId"`
	ExpiresAt string          `json:"expiresAt"`
	Emails    []BehaviorValue `json:"emails"`
}
