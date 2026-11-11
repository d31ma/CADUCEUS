// @Service
type SeenEventsService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
	// @Needs
	timestamps *TimestampService
}

func (seen *SeenEventsService) Seen(domainId string, eventId string) (bool, error) {
	found := []DocumentId{}
	failure := seen.documents.Matching(seen.names.SeenEvents(domainId), seen.byEvent(eventId), 1, &found)
	return len(found) > 0, failure
}

func (seen *SeenEventsService) Remember(domainId string, eventId string) error {
	record := SeenEventRecord{DomainId: domainId, EventId: eventId, SeenAt: seen.timestamps.Now()}
	_, failure := seen.documents.InsertIfAbsent(seen.names.SeenEvents(domainId), seen.byEvent(eventId), record)
	return failure
}

func (seen *SeenEventsService) byEvent(eventId string) map[string]any {
	return map[string]any{"eventId": eventId}
}
