import (
	"cmp"

	"mesail.dev/packages/core"
)

// @Service
type ConversationStoreService struct {
	// @Needs
	tenant *TenantCollectionsService
	// @Needs
	names *CollectionNamesService
}

func (store *ConversationStoreService) Insert(conversation core.Conversation) (string, error) {
	return store.tenant.Insert(store.names.Conversations(conversation.DomainId), conversation)
}

func (store *ConversationStoreService) Save(conversation core.Conversation) error {
	return store.tenant.Replace(store.names.Conversations(conversation.DomainId), conversation.Id, conversation)
}

func (store *ConversationStoreService) Find(domainId string, storedPart string) (core.Conversation, bool, error) {
	conversation := core.Conversation{}
	found, failure := store.tenant.Find(store.names.Conversations(domainId), domainId, storedPart, &conversation)
	keyed, keyFound, keyFailure := store.byKeyUnless(found, failure, domainId, storedPart)
	chosen, chosenFound := store.chosen(found, conversation, keyed, keyFound)
	return chosen, chosenFound, cmp.Or(failure, keyFailure)
}

func (store *ConversationStoreService) ByChannel(domainId string, channelId string) (core.Conversation, bool, error) {
	return store.first(domainId, map[string]any{store.channelField(): channelId})
}

func (store *ConversationStoreService) ByParticipantKey(domainId string, participantKey string) (core.Conversation, bool, error) {
	return store.first(domainId, map[string]any{store.participantKeyField(): participantKey})
}

func (store *ConversationStoreService) first(domainId string, where map[string]any) (core.Conversation, bool, error) {
	conversation := core.Conversation{}
	found, failure := store.tenant.First(store.names.Conversations(domainId), domainId, where, &conversation)
	return conversation, found, failure
}

func (store *ConversationStoreService) byKeyUnless(found bool, failure error, domainId string, storedPart string) (core.Conversation, bool, error) {
	if found || failure != nil {
		return core.Conversation{}, false, nil
	}
	return store.first(domainId, map[string]any{store.keyField(): storedPart})
}

func (store *ConversationStoreService) chosen(found bool, byId core.Conversation, byKey core.Conversation, keyFound bool) (core.Conversation, bool) {
	if found {
		return byId, true
	}
	return byKey, keyFound
}

func (store *ConversationStoreService) keyField() string {
	return "key"
}

func (store *ConversationStoreService) channelField() string {
	return "channelId"
}

func (store *ConversationStoreService) participantKeyField() string {
	return "participantKey"
}
