package storage

import "mesail.dev/packages/core"

type Messages struct {
	Tenant Tenant
	Names  CollectionNames
}

func (store Messages) Find(domainId string, month string, id string) (core.Message, bool, error) {
	message := core.Message{}
	found, failure := store.Tenant.Find(store.Names.Messages(domainId, month), domainId, id, &message)
	return message, found, failure
}

func (store Messages) BySendKey(domainId string, month string, sendKey string) (core.Message, bool, error) {
	message := core.Message{}
	found, failure := store.Tenant.First(store.Names.Messages(domainId, month), domainId, map[string]any{store.sendKeyField(): sendKey}, &message)
	return message, found, failure
}

func (store Messages) OfConversation(domainId string, month string, conversationId string) ([]core.Message, error) {
	messages := []core.Message{}
	failure := store.Tenant.Matching(store.Names.Messages(domainId, month), domainId, map[string]any{store.conversationField(): conversationId}, &messages)
	return messages, failure
}

func (store Messages) Insert(message core.Message) (string, error) {
	return store.Tenant.InsertTracked(store.Names.Messages(message.DomainId, message.Month), message, message.OutboxPending)
}

func (store Messages) Save(message core.Message) error {
	return store.Tenant.ReplaceTracked(store.Names.Messages(message.DomainId, message.Month), message.Id, message, message.OutboxPending)
}

func (Messages) sendKeyField() string {
	return "sendKey"
}

func (Messages) conversationField() string {
	return "conversationId"
}
