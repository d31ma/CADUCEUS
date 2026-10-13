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
	return helper.store().FindById(undoId)
}

// Delete deletes one outbox document.
func (helper *OutboxHelper) Delete(documentId string) error {
	return helper.store().Delete(documentId)
}

// FindDue answers the entries whose next delivery attempt is due at now
// (milliseconds). Entries written before the retry fields existed fall back
// to expiresAt.
func (helper *OutboxHelper) FindDue(now int64) ([]FyloRecord, error) {
	documents, err := helper.expiredBy(now)
	if err != nil {
		return nil, err
	}
	due := []FyloRecord{}
	for _, document := range documents {
		entry := document.Data
		at, isNumber := RecordValues{}.Number(RecordValues{}.Coalesce(entry["nextAttempt"], entry[helper.expiryField()]))
		if isNumber && at <= float64(now) {
			due = append(due, document)
		}
	}
	return due, nil
}

// ListAll answers every outbox entry, in FYLO's order.
func (helper *OutboxHelper) ListAll() ([]FyloRecord, error) {
	return helper.store().All()
}

// Update merges a patch into one outbox entry.
func (helper *OutboxHelper) Update(documentId string, patch map[string]any) error {
	return helper.store().Patch(documentId, patch)
}

func (helper *OutboxHelper) expiredBy(now int64) ([]FyloRecord, error) {
	conditions := FyloConditions{}
	return conditions.Matching(helper.store(), conditions.Condition(helper.expiryField(), "$lte", now))
}

func (OutboxHelper) expiryField() string { return "expiresAt" }

func (helper *OutboxHelper) store() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Outbox}
}
