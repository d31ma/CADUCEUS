package core

type ConversationSync struct {
	ConversationId string
	State          string
	Messages       []MessageView
	Next           string
	More           bool
}
