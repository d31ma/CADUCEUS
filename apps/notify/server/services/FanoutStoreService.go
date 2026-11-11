// @Service
type FanoutStoreService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (store *FanoutStoreService) InsertIfAbsent(record FanoutRecord) (FanoutRecord, error) {
	record.State = store.waiting()
	id, failure := store.documents.InsertIfAbsent(store.names.Fanout(record.DomainId), map[string]any{"messageId": record.MessageId}, record)
	record.Id = id
	return record, failure
}

func (store *FanoutStoreService) Waiting(domainId string, limit int) ([]FanoutRecord, error) {
	found := []FanoutRecord{}
	failure := store.documents.Matching(store.names.Fanout(domainId), map[string]any{"state": store.waiting()}, limit, &found)
	return found, failure
}

func (store *FanoutStoreService) SaveRecipients(record FanoutRecord, recipients []string) error {
	patch := FanoutRecipientsPatch{Recipients: recipients, State: store.stateFor(recipients)}
	return store.documents.Patch(store.names.Fanout(record.DomainId), record.Id, patch)
}

func (store *FanoutStoreService) stateFor(recipients []string) string {
	if len(recipients) == 0 {
		return "done"
	}
	return store.waiting()
}

func (store *FanoutStoreService) waiting() string {
	return "waiting"
}
