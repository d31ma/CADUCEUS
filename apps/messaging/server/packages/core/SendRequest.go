package core

type SendRequest struct {
	ClientMessageId    string
	ConversationId     string
	ThreadRootId       string
	AlsoToConversation bool
	Body               string
	AttachmentIds      []string
	Recipients         []RecipientChoice
	Email              EmailOptions
}
