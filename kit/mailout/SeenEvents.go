package mailout

import "time"

type SeenEvents struct {
	Store DocumentStore
	Now   func() time.Time
}

func (SeenEvents) base() string { return "events-seen" }

func (seen SeenEvents) Seen(domainId string, eventId string) (bool, error) {
	collection, err := seen.Store.Ready(seen.base(), domainId)
	if err != nil {
		return false, err
	}
	return seen.Store.Exists(collection, map[string]any{"eventId": eventId})
}

func (seen SeenEvents) Remember(domainId string, eventId string) error {
	collection, err := seen.Store.Ready(seen.base(), domainId)
	if err != nil {
		return err
	}
	_, err = seen.Store.Documents.Insert(collection, SeenEvent{DomainId: domainId, EventId: eventId, SeenAt: Clock{}.Format(seen.Now())})
	return err
}
