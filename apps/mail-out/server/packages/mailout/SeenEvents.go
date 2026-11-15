package mailout

import "time"

type SeenEvents struct {
	Store DocumentStore
	Now   func() time.Time
}

func (SeenEvents) base() string { return "events-seen" }

func (SeenEvents) eventField() string { return "eventId" }

func (seen SeenEvents) Seen(domainId string, eventId string) (bool, error) {
	collection := seen.Store.Name(seen.base(), domainId)
	return seen.Store.Exists(collection, map[string]any{seen.eventField(): eventId})
}

func (seen SeenEvents) Remember(domainId string, eventId string) error {
	collection := seen.Store.Name(seen.base(), domainId)
	_, err := seen.Store.Insert(collection, SeenEvent{DomainId: domainId, EventId: eventId, SeenAt: Clock{}.Format(seen.Now())})
	return err
}
