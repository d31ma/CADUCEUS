type TemplateInsertRequested struct {
	TemplateID string `json:"templateId"`
	Name       string `json:"name"`
	Text       string `json:"text"`
	To         string `json:"to"`
	Cc         string `json:"cc"`
	Subject    string `json:"subject"`
	Confirm    bool   `json:"confirm"`
}
