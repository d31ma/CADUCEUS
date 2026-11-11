package core

type ConversationStore interface {
	Insert(conversation Conversation) (string, error)
	Save(conversation Conversation) error
	Find(domainId string, storedPart string) (Conversation, bool, error)
	ByChannel(domainId string, channelId string) (Conversation, bool, error)
	ByParticipantKey(domainId string, participantKey string) (Conversation, bool, error)
}
