import "mesail.dev/packages/core"

// @Service
type MessageStoreService struct {
	// @Needs
	tenant *TenantCollectionsService
	// @Needs
	names *CollectionNamesService
}

func (store *MessageStoreService) Find(domainId string, month string, id string) (core.Message, bool, error) {
	message := core.Message{}
	found, failure := store.tenant.Find(store.names.Messages(domainId, month), domainId, id, &message)
	return message, found, failure
}

func (store *MessageStoreService) BySendKey(domainId string, month string, sendKey string) (core.Message, bool, error) {
	message := core.Message{}
	found, failure := store.tenant.First(store.names.Messages(domainId, month), domainId, map[string]any{store.sendKeyField(): sendKey}, &message)
	return message, found, failure
}

func (store *MessageStoreService) OfConversation(domainId string, month string, conversationId string) ([]core.Message, error) {
	messages := []core.Message{}
	failure := store.tenant.Matching(store.names.Messages(domainId, month), domainId, map[string]any{store.conversationField(): conversationId}, &messages)
	return messages, failure
}

func (store *MessageStoreService) Insert(message core.Message) (string, error) {
	return store.tenant.InsertTracked(store.names.Messages(message.DomainId, message.Month), message, message.OutboxPending)
}

func (store *MessageStoreService) Save(message core.Message) error {
	return store.tenant.ReplaceTracked(store.names.Messages(message.DomainId, message.Month), message.Id, message, message.OutboxPending)
}

func (store *MessageStoreService) sendKeyField() string {
	return "sendKey"
}

func (store *MessageStoreService) conversationField() string {
	return "conversationId"
}
