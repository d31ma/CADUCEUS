package search

type Document struct {
	DomainId       string   `json:"domainId"`
	MessageId      string   `json:"messageId"`
	ConversationId string   `json:"conversationId"`
	AuthorMemberId string   `json:"authorMemberId"`
	AuthorAddress  string   `json:"authorAddress"`
	AuthorName     string   `json:"authorName"`
	Text           string   `json:"text"`
	Truncated      bool     `json:"truncated"`
	Subject        string   `json:"subject"`
	FileNames      []string `json:"fileNames"`
	HasAttachment  bool     `json:"hasAttachment"`
	IsEmail        bool     `json:"isEmail"`
	CreatedAt      string   `json:"createdAt"`
	EditedAt       string   `json:"editedAt"`
	Deleted        bool     `json:"deleted"`
	Terms          []string `json:"terms"`
}
