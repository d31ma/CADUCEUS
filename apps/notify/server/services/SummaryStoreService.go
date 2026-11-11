// @Service
type SummaryStoreService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
	// @Needs
	registry *OutboxCollectionsService
	// @Needs
	outboxStates *OutboxStatesService
}

func (store *SummaryStoreService) InsertIfAbsent(record SummaryRecord) (SummaryRecord, error) {
	collection := store.Collection(record.DomainId)
	failure := store.registry.Register(collection)
	record.OutboxState, record.DeadState = store.outboxStates.Pending(record.OutboxPending), store.outboxStates.Dead(record.OutboxDead)
	record.Id, failure = store.insertUnless(failure, collection, record)
	return record, failure
}

func (store *SummaryStoreService) Find(domainId string, storedId string) (SummaryRecord, bool, error) {
	record := SummaryRecord{}
	found, failure := store.documents.Find(store.Collection(domainId), storedId, &record)
	return record, found, failure
}

func (store *SummaryStoreService) Collection(domainId string) string {
	return store.names.Summaries(domainId)
}

func (store *SummaryStoreService) insertUnless(failure error, collection string, record SummaryRecord) (string, error) {
	if failure != nil {
		return "", failure
	}
	return store.documents.InsertIfAbsent(collection, map[string]any{"batchKey": record.BatchKey}, record)
}
