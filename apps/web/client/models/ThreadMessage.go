type ThreadMessage struct {
	MessageId          string           `json:"messageId"`
	ConversationId     string           `json:"conversationId"`
	ThreadRootId       string           `json:"threadRootId,omitempty"`
	AlsoToConversation bool             `json:"alsoToConversation"`
	Author             ThreadAuthor     `json:"author"`
	Body               string           `json:"body"`
	BodyFormat         string           `json:"bodyFormat"`
	Truncated          bool             `json:"truncated"`
	Reactions          map[string]int   `json:"reactions"`
	MyReactions        []string         `json:"myReactions"`
	ReplyCount         int              `json:"replyCount"`
	LastReplyAt        string           `json:"lastReplyAt,omitempty"`
	EditedAt           string           `json:"editedAt,omitempty"`
	DeletedAt          string           `json:"deletedAt,omitempty"`
	CreatedAt          string           `json:"createdAt"`
	Deliveries         []ThreadDelivery `json:"deliveries"`
}
