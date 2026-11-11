type SearchHitFilter struct {
	Query          string `json:"query"`
	ConversationId string `json:"conversationId"`
	Author         string `json:"author"`
	From           string `json:"from"`
	To             string `json:"to"`
	HasAttachment  bool   `json:"hasAttachment"`
	IsEmail        bool   `json:"isEmail"`
	Cursor         string `json:"cursor"`
	Limit          string `json:"limit"`
}
