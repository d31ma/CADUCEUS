type ConversationViewContext struct {
	MyAddress      string   `json:"myAddress"`
	DomainId       string   `json:"domainId"`
	Section        string   `json:"section"`
	UnreadCount    int      `json:"unreadCount"`
	NowMs          float64  `json:"nowMs"`
	TimeZone       string   `json:"timeZone"`
	AllowedSenders []string `json:"allowedSenders"`
	ShowQuoted     []string `json:"showQuoted"`
	EditingId      string   `json:"editingId"`
}
