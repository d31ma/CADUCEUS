type EmailOptions struct {
	Subject        string `json:"subject"`
	FromAddress    string `json:"fromAddress,omitempty"`
	SendToExternal bool   `json:"sendToExternal"`
}
