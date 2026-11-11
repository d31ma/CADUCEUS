type ConversationMembers struct {
	ID             string   `json:"id,omitempty"`
	DomainId       string   `json:"domainId"`
	ConversationId string   `json:"conversationId"`
	MemberIds      []string `json:"memberIds"`
}
