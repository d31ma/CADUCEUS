package events

type MessageCreated struct {
	MessageId          string   `json:"messageId"`
	ConversationId     string   `json:"conversationId"`
	ConversationKind   string   `json:"conversationKind"`
	ThreadRootId       string   `json:"threadRootId"`
	AuthorKind         string   `json:"authorKind"`
	AuthorMemberId     string   `json:"authorMemberId"`
	AuthorAddress      string   `json:"authorAddress"`
	AuthorDisplayName  string   `json:"authorDisplayName"`
	Body               string   `json:"body"`
	Subject            string   `json:"subject"`
	AttachmentIds      []string `json:"attachmentIds"`
	FileNames          []string `json:"fileNames"`
	IsEmail            bool     `json:"isEmail"`
	MentionedMemberIds []string `json:"mentionedMemberIds"`
	MemberIds          []string `json:"memberIds"`
	CreatedAt          string   `json:"createdAt"`
}
