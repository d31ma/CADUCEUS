package core

type ReactionStore interface {
	Insert(reaction Reaction) (string, error)
	Save(reaction Reaction) error
	ByKey(domainId string, key string) (Reaction, bool, error)
	OfConversation(domainId string, conversationId string) ([]Reaction, error)
}
