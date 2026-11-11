type SearchHitQuery struct {
	Authorization  string `json:"authorization"`
	Query          string `json:"query"`
	ConversationId string `json:"conversationId"`
	Author         string `json:"author"`
	From           string `json:"from"`
	To             string `json:"to"`
	HasAttachment  string `json:"hasAttachment"`
	IsEmail        string `json:"isEmail"`
	Cursor         string `json:"cursor"`
	Limit          string `json:"limit"`
}
