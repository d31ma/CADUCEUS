package search

type SeenEvents struct {
	Store DocumentStore
}

func (seen SeenEvents) Seen(domainId string, eventId string) (bool, error) {
	found := []SeenEvent{}
	err := seen.Store.All(Collections{}.SeenEvents(domainId), map[string]any{"domainId": domainId, "eventId": eventId}, &found)
	return len(found) > 0, err
}

func (seen SeenEvents) Remember(domainId string, eventId string) error {
	_, err := seen.Store.Insert(Collections{}.SeenEvents(domainId), SeenEvent{DomainId: domainId, EventId: eventId})
	return err
}
