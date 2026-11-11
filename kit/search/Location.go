package search

type Location struct {
	DomainId   string `json:"domainId"`
	MessageId  string `json:"messageId"`
	Month      string `json:"month"`
	DocumentId string `json:"documentId"`
	Deleted    bool   `json:"deleted"`
}
