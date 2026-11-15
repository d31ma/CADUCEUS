package core

type ChangeHeadStore interface {
	Find(domainId string, conversationId string) (ChangeHead, bool, error)
	Insert(head ChangeHead) (string, error)
	Save(head ChangeHead) error
}
