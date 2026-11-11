import "mesail.dev/packages/core"

// @Service
type ThreadKeyStoreService struct {
	// @Needs
	tenant *TenantCollectionsService
	// @Needs
	names *CollectionNamesService
}

func (store *ThreadKeyStoreService) Insert(threadKey core.ThreadKey) (string, error) {
	return store.tenant.Insert(store.names.ThreadKeys(threadKey.DomainId), threadKey)
}

func (store *ThreadKeyStoreService) ByKey(domainId string, key string) (core.ThreadKey, bool, error) {
	threadKey := core.ThreadKey{}
	found, failure := store.tenant.First(store.names.ThreadKeys(domainId), domainId, map[string]any{store.keyField(): key}, &threadKey)
	return threadKey, found, failure
}

func (store *ThreadKeyStoreService) keyField() string {
	return "key"
}
