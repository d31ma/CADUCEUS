import (
	"errors"
	"time"
)

// @Delegate
// @NeedsAll()
type SeenEventsDelegate struct {
	dataSource DataSource
}

func (SeenEventsDelegate) collection() string { return "events-seen" }

func (SeenEventsDelegate) oneDocument() int { return 1 }

func (seen *SeenEventsDelegate) Seen(domainId string, eventId string) (bool, error) {
	collection, err := seen.ready(domainId)
	window, scrollErr := seen.dataSource.Scroll(collection, map[string]any{"eventId": eventId}, "", seen.oneDocument())
	found := []SeenEvent{}
	decodeErr := window.Decode(&found)
	return len(found) > 0, errors.Join(err, scrollErr, decodeErr)
}

func (seen *SeenEventsDelegate) Remember(domainId string, eventId string) error {
	collection, err := seen.ready(domainId)
	_, insertErr := seen.dataSource.Insert(collection, SeenEvent{DomainId: domainId, EventId: eventId, SeenAt: TimeUtils{}.Format(time.Now())})
	return errors.Join(err, insertErr)
}

func (seen *SeenEventsDelegate) ready(domainId string) (string, error) {
	collection := StorageUtils{}.CollectionName(seen.collection(), domainId)
	return collection, seen.dataSource.CreateCollection(collection)
}
