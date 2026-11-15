package storage

import "mesail.dev/packages/core"

type ChangeHeads struct {
	Tenant Tenant
	Names  CollectionNames
}

func (store ChangeHeads) Find(domainId string, conversationId string) (core.ChangeHead, bool, error) {
	head := core.ChangeHead{}
	found, failure := store.Tenant.First(store.Names.ChangeHeads(domainId), domainId, map[string]any{store.conversationField(): conversationId}, &head)
	return head, found, failure
}

func (store ChangeHeads) Insert(head core.ChangeHead) (string, error) {
	return store.Tenant.Insert(store.Names.ChangeHeads(head.DomainId), head)
}

func (store ChangeHeads) Save(head core.ChangeHead) error {
	return store.Tenant.Replace(store.Names.ChangeHeads(head.DomainId), head.Id, head)
}

func (ChangeHeads) conversationField() string {
	return "conversationId"
}
