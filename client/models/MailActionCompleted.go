type MailActionCompleted struct {
	ActionId  string `json:"actionId"`
	ExpiresAt string `json:"expiresAt"`
	Message   string `json:"message"`
}
