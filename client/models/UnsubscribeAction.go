type UnsubscribeAction struct {
	Type    string   `json:"type"`
	Url     string   `json:"url"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}
