package backups

import "mesail.dev/packages/collections"

type SeenEvents struct {
	Documents Documents
	Scroll    Scroller
	Known     collections.KnownCollections
}

func (seen SeenEvents) Seen(domainId string, eventId string) (bool, error) {
	collection := seen.collection(domainId)
	found := false
	err := seen.Known.Read(seen.Documents, collection, func() error {
		var failure error
		found, failure = Pages{Scroll: seen.Scroll}.First(collection, map[string]any{"eventId": eventId}, &SeenEvent{})
		return failure
	})
	return found, err
}

func (seen SeenEvents) Remember(domainId string, eventId string) error {
	collection := seen.collection(domainId)
	return seen.Known.Write(seen.Documents, collection, func() error {
		_, failure := seen.Documents.Insert(collection, SeenEvent{DomainId: domainId, EventId: eventId})
		return failure
	})
}

func (SeenEvents) collection(domainId string) string {
	return collections.CollectionName{}.ForDomain("backup-events", domainId)
}
