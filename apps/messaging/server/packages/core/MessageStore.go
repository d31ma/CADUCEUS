package core

type MessageStore interface {
	Insert(message Message) (string, error)
	Save(message Message) error
	Find(domainId string, month string, id string) (Message, bool, error)
	BySendKey(domainId string, month string, sendKey string) (Message, bool, error)
	OfConversation(domainId string, month string, conversationId string) ([]Message, error)
}
