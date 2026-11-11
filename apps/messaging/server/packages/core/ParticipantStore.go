package core

type ParticipantStore interface {
	Insert(participant Participant) (string, error)
	Save(participant Participant) error
	Delete(domainId string, id string) error
	OfConversation(domainId string, conversationId string) ([]Participant, error)
	OfMember(domainId string, memberId string) ([]Participant, error)
}
