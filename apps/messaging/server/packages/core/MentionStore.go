package core

type MentionStore interface {
	Insert(mention Mention) (string, error)
	OfMember(domainId string, memberId string) ([]Mention, error)
}
