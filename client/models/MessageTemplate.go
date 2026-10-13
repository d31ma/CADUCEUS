type MessageTemplate struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
	To      string `json:"to"`
	Cc      string `json:"cc"`
}
