type MailActionRequested struct {
	Ids     []string       `json:"ids"`
	Patch   map[string]any `json:"patch"`
	Message string         `json:"message"`
}
