package storage

import "mesail.dev/packages/core"

type Changes struct {
	Tenant Tenant
	Names  CollectionNames
	Logs   ChangeLogs
}

func (store Changes) Append(change core.Change) (string, error) {
	collection := store.Names.Changes(change.DomainId, change.Month)
	err := store.Logs.Register(collection, change.Month)
	if err != nil {
		return "", err
	}
	return store.Tenant.Insert(collection, change)
}

func (store Changes) OfEvent(domainId string, month string, eventId string) ([]core.Change, error) {
	return store.matching(domainId, month, map[string]any{store.eventField(): eventId})
}

func (store Changes) OfConversation(domainId string, month string, conversationId string) ([]core.Change, error) {
	return store.matching(domainId, month, map[string]any{store.conversationField(): conversationId})
}

func (store Changes) matching(domainId string, month string, where map[string]any) ([]core.Change, error) {
	changes := []core.Change{}
	failure := store.Tenant.Matching(store.Names.Changes(domainId, month), domainId, where, &changes)
	return changes, failure
}

func (Changes) eventField() string {
	return "eventId"
}

func (Changes) conversationField() string {
	return "conversationId"
}
