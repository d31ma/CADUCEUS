type ComposePrefill struct {
	To            string   `json:"to"`
	Subject       string   `json:"subject"`
	Body          string   `json:"body"`
	InReplyTo     string   `json:"inReplyTo"`
	References    []string `json:"references"`
	SourceEmailId string   `json:"sourceEmailId"`
	DraftId       string   `json:"draftId"`
}
