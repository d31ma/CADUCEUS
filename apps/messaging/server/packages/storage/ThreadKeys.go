package storage

import "mesail.dev/packages/core"

type ThreadKeys struct {
	Tenant Tenant
	Names  CollectionNames
}

func (store ThreadKeys) Insert(threadKey core.ThreadKey) (string, error) {
	return store.Tenant.Insert(store.Names.ThreadKeys(threadKey.DomainId), threadKey)
}

func (store ThreadKeys) ByKey(domainId string, key string) (core.ThreadKey, bool, error) {
	threadKey := core.ThreadKey{}
	found, failure := store.Tenant.First(store.Names.ThreadKeys(domainId), domainId, map[string]any{store.keyField(): key}, &threadKey)
	return threadKey, found, failure
}

func (ThreadKeys) keyField() string {
	return "key"
}
