type ConversationSearch struct {
	Term     string                `json:"term"`
	Messages []ConversationMessage `json:"messages"`
	Results  []FindResult          `json:"results"`
	Index    int                   `json:"index"`
	Count    int                   `json:"count"`
}
