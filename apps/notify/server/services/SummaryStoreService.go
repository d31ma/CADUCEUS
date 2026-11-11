// @Service
type SummaryStoreService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
	// @Needs
	registry *OutboxCollectionsService
}

func (store *SummaryStoreService) InsertIfAbsent(record SummaryRecord) (SummaryRecord, error) {
	collection := store.Collection(record.DomainId)
	failure := store.registry.Register(collection)
	record.OutboxWaiting, record.OutboxHasDead = record.OutboxPending > 0, record.OutboxDead > 0
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
