type Message struct {
	MessageId      string               `json:"messageId"`
	ConversationId string               `json:"conversationId"`
	ThreadRootId   string               `json:"threadRootId,omitempty"`
	Author         Author               `json:"author"`
	Body           string               `json:"body"`
	BodyFormat     string               `json:"bodyFormat"`
	Blocks         []Block              `json:"blocks"`
	Truncated      bool                 `json:"truncated"`
	Attachments    []AttachmentRef      `json:"attachments"`
	Email          *MessageEmailSummary `json:"email,omitempty"`
	Reactions      map[string]int       `json:"reactions"`
	MyReactions    []string             `json:"myReactions"`
	ReplyCount     int                  `json:"replyCount"`
	LastReplyAt    string               `json:"lastReplyAt,omitempty"`
	EditedAt       string               `json:"editedAt,omitempty"`
	DeletedAt      string               `json:"deletedAt,omitempty"`
	CreatedAt      string               `json:"createdAt"`
	Deliveries     []DeliveryStatus     `json:"deliveries"`
}
