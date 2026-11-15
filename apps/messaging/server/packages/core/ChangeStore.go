package core

type ChangeStore interface {
	Append(change Change) (string, error)
	OfEvent(domainId string, month string, eventId string) ([]Change, error)
	OfConversation(domainId string, month string, conversationId string) ([]Change, error)
}
