package storage

type SeenEvents struct {
	Tenant Tenant
	Names  CollectionNames
}

func (store SeenEvents) Seen(domainId string, eventId string) (bool, error) {
	return store.Tenant.First(store.Names.Seen(domainId), domainId, map[string]any{store.eventField(): eventId}, &SeenEvent{})
}

func (store SeenEvents) Remember(domainId string, eventId string) error {
	_, failure := store.Tenant.Insert(store.Names.Seen(domainId), SeenEvent{DomainId: domainId, EventId: eventId})
	return failure
}

func (SeenEvents) eventField() string {
	return "eventId"
}
