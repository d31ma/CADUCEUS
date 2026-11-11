package core

type ThreadKeyStore interface {
	Insert(threadKey ThreadKey) (string, error)
	ByKey(domainId string, key string) (ThreadKey, bool, error)
}
