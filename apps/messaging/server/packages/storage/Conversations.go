package storage

import (
	"cmp"

	"mesail.dev/packages/core"
)

type Conversations struct {
	Tenant Tenant
	Names  CollectionNames
}

func (store Conversations) Insert(conversation core.Conversation) (string, error) {
	return store.Tenant.Insert(store.Names.Conversations(conversation.DomainId), conversation)
}

func (store Conversations) Save(conversation core.Conversation) error {
	return store.Tenant.Replace(store.Names.Conversations(conversation.DomainId), conversation.Id, conversation)
}

func (store Conversations) Find(domainId string, storedPart string) (core.Conversation, bool, error) {
	conversation := core.Conversation{}
	found, failure := store.Tenant.Find(store.Names.Conversations(domainId), domainId, storedPart, &conversation)
	keyed, keyFound, keyFailure := store.byKeyUnless(found, failure, domainId, storedPart)
	chosen, chosenFound := store.chosen(found, conversation, keyed, keyFound)
	return chosen, chosenFound, cmp.Or(failure, keyFailure)
}

func (store Conversations) ByChannel(domainId string, channelId string) (core.Conversation, bool, error) {
	return store.first(domainId, map[string]any{store.channelField(): channelId})
}

func (store Conversations) ByParticipantKey(domainId string, participantKey string) (core.Conversation, bool, error) {
	return store.first(domainId, map[string]any{store.participantKeyField(): participantKey})
}

func (store Conversations) first(domainId string, where map[string]any) (core.Conversation, bool, error) {
	conversation := core.Conversation{}
	found, failure := store.Tenant.First(store.Names.Conversations(domainId), domainId, where, &conversation)
	return conversation, found, failure
}

func (store Conversations) byKeyUnless(found bool, failure error, domainId string, storedPart string) (core.Conversation, bool, error) {
	if found || failure != nil {
		return core.Conversation{}, false, nil
	}
	return store.first(domainId, map[string]any{store.keyField(): storedPart})
}

func (Conversations) chosen(found bool, byId core.Conversation, byKey core.Conversation, keyFound bool) (core.Conversation, bool) {
	if found {
		return byId, true
	}
	return byKey, keyFound
}

func (Conversations) keyField() string {
	return "key"
}

func (Conversations) channelField() string {
	return "channelId"
}

func (Conversations) participantKeyField() string {
	return "participantKey"
}
