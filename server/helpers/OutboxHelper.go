// OutboxHelper keeps the outbound queue: messages waiting for their undo
// window or their next delivery attempt.
// @Helper
type OutboxHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores one outbox entry and answers its document id.
func (helper *OutboxHelper) Put(entry map[string]any) (string, error) {
	return helper.store().Put(entry)
}

// FindById answers the document id and entry with the undo id, or "" and nil.
func (helper *OutboxHelper) FindById(undoId string) (string, map[string]any, error) {
	documents, err := helper.store().Equal("id", undoId)
	id, entry := StoredRecords{}.First(documents)
	return id, entry, err
}

// Delete deletes one outbox document.
func (helper *OutboxHelper) Delete(documentId string) error {
	return helper.store().Delete(documentId)
}

// FindExpired answers the entries whose undo window has passed, in FYLO's
// order (the former {docId: entry} record).
func (helper *OutboxHelper) FindExpired() ([]FyloDocument, error) {
	return helper.expiredBy(EpochTime{}.Now())
}

// FindDue answers the entries whose next delivery attempt is due at now
// (milliseconds). Entries written before the retry fields existed fall back
// to expiresAt.
func (helper *OutboxHelper) FindDue(now int64) ([]FyloDocument, error) {
	documents, err := helper.expiredBy(now)
	if err != nil {
		return nil, err
	}
	due := []FyloDocument{}
	for _, document := range documents {
		entry := StoredRecords{}.Decoded(document.Data)
		at, isNumber := RecordValues{}.Number(RecordValues{}.Coalesce(entry["nextAttempt"], entry[helper.expiryField()]))
		if isNumber && at <= float64(now) {
			due = append(due, document)
		}
	}
	return due, nil
}

// ListAll answers every outbox entry, in FYLO's order.
func (helper *OutboxHelper) ListAll() ([]FyloDocument, error) {
	return helper.store().All()
}

// Update merges a patch into one outbox entry.
func (helper *OutboxHelper) Update(documentId string, patch map[string]any) error {
	return helper.store().Patch(documentId, patch)
}

func (helper *OutboxHelper) expiredBy(now int64) ([]FyloDocument, error) {
	store := helper.store()
	return store.Find(store.Condition(helper.expiryField(), "$lte", now))
}

func (OutboxHelper) expiryField() string { return "expiresAt" }

func (helper *OutboxHelper) store() CollectionStore {
	return CollectionStore{Database: helper.fyloHelper.Database(), Collection: FyloCollections{}.Named().Outbox}
}
