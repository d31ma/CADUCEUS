package core

type Message struct {
	Id                 string    `json:"id,omitempty"`
	DomainId           string    `json:"domainId"`
	ConversationId     string    `json:"conversationId"`
	ThreadRootId       string    `json:"threadRootId"`
	AlsoToConversation bool      `json:"alsoToConversation"`
	Month              string    `json:"month"`
	Sequence           int       `json:"sequence"`
	AuthorKind         string    `json:"authorKind"`
	AuthorMemberId     string    `json:"authorMemberId"`
	AuthorAddress      string    `json:"authorAddress"`
	AuthorDisplayName  string    `json:"authorDisplayName"`
	ClientMessageId    string    `json:"clientMessageId"`
	SendKey            string    `json:"sendKey"`
	Body               string    `json:"body"`
	BodyFormat         string    `json:"bodyFormat"`
	BlocksJson         string    `json:"blocksJson"`
	Truncated          bool      `json:"truncated"`
	AttachmentIds      []string  `json:"attachmentIds"`
	AttachmentsJson    string    `json:"attachmentsJson"`
	MentionedMemberIds []string  `json:"mentionedMemberIds"`
	IsEmail            bool      `json:"isEmail"`
	Email              EmailMeta `json:"email"`
	Deliveries         []string  `json:"deliveries"`
	EventsStaged       bool      `json:"eventsStaged"`
	ReplyCount         int       `json:"replyCount"`
	LastReplyAt        string    `json:"lastReplyAt"`
	EditedAt           string    `json:"editedAt"`
	DeletedAt          string    `json:"deletedAt"`
	CreatedAt          string    `json:"createdAt"`
	OutboxFields
}
