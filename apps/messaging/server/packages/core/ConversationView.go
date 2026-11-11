package core

type ConversationView struct {
	Conversation   Conversation
	ConversationId string
	Title          string
	UnreadCount    int
	MentionCount   int
}
