import "mesail.dev/packages/core"

// @Service
type ReactionStoreService struct {
	// @Needs
	names *CollectionNamesService
	// @Needs
	tenant *TenantCollectionsService
}

func (store *ReactionStoreService) ByKey(domainId string, key string) (core.Reaction, bool, error) {
	reaction := core.Reaction{}
	found, failure := store.tenant.First(store.names.Reactions(domainId), domainId, map[string]any{store.keyField(): key}, &reaction)
	return reaction, found, failure
}

func (store *ReactionStoreService) Insert(reaction core.Reaction) (string, error) {
	return store.tenant.InsertTracked(store.names.Reactions(reaction.DomainId), reaction, reaction.OutboxPending)
}

func (store *ReactionStoreService) OfConversation(domainId string, conversationId string) ([]core.Reaction, error) {
	reactions := []core.Reaction{}
	failure := store.tenant.Matching(store.names.Reactions(domainId), domainId, map[string]any{store.conversationField(): conversationId}, &reactions)
	return reactions, failure
}

func (store *ReactionStoreService) Save(reaction core.Reaction) error {
	return store.tenant.ReplaceTracked(store.names.Reactions(reaction.DomainId), reaction.Id, reaction, reaction.OutboxPending)
}

func (store *ReactionStoreService) keyField() string {
	return "key"
}

func (store *ReactionStoreService) conversationField() string {
	return "conversationId"
}
