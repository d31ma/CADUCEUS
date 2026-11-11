type EmailOptions struct {
	Subject        string `json:"subject,omitempty"`
	FromAddress    string `json:"fromAddress,omitempty"`
	SendToExternal bool   `json:"sendToExternal,omitempty"`
}
