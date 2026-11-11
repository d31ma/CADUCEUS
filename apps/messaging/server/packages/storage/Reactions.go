package storage

import "mesail.dev/packages/core"

type Reactions struct {
	Tenant Tenant
	Names  CollectionNames
}

func (store Reactions) ByKey(domainId string, key string) (core.Reaction, bool, error) {
	reaction := core.Reaction{}
	found, failure := store.Tenant.First(store.Names.Reactions(domainId), domainId, map[string]any{store.keyField(): key}, &reaction)
	return reaction, found, failure
}

func (store Reactions) Insert(reaction core.Reaction) (string, error) {
	return store.Tenant.InsertTracked(store.Names.Reactions(reaction.DomainId), reaction, reaction.OutboxPending)
}

func (store Reactions) OfConversation(domainId string, conversationId string) ([]core.Reaction, error) {
	reactions := []core.Reaction{}
	failure := store.Tenant.Matching(store.Names.Reactions(domainId), domainId, map[string]any{store.conversationField(): conversationId}, &reactions)
	return reactions, failure
}

func (store Reactions) Save(reaction core.Reaction) error {
	return store.Tenant.ReplaceTracked(store.Names.Reactions(reaction.DomainId), reaction.Id, reaction, reaction.OutboxPending)
}

func (Reactions) keyField() string {
	return "key"
}

func (Reactions) conversationField() string {
	return "conversationId"
}
