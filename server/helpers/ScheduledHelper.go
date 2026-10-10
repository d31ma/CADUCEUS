// ScheduledHelper keeps messages scheduled to send later.
// @Helper
type ScheduledHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores one scheduled entry and answers its document id.
func (helper *ScheduledHelper) Put(entry map[string]any) (string, error) {
	return helper.store().Put(entry)
}

// ListForOwner answers the entries of one owner (entries written before the
// owner field fall back to their sender), in FYLO's order.
func (helper *ScheduledHelper) ListForOwner(owner string) ([]FyloRecord, error) {
	documents, err := helper.ListAll()
	if err != nil {
		return nil, err
	}
	owned := []FyloRecord{}
	for _, document := range documents {
		entry := document.Data
		if (RecordValues{}).Coalesce(entry["owner"], entry["sender"]) == owner {
			owned = append(owned, document)
		}
	}
	return owned, nil
}

// ListAll answers every scheduled entry, for the system worker that
// reconciles entries written before the queue existed.
func (helper *ScheduledHelper) ListAll() ([]FyloRecord, error) {
	return helper.store().All()
}

// FindById answers the document id and entry with the id, or "" and nil.
func (helper *ScheduledHelper) FindById(id string) (string, map[string]any, error) {
	return helper.store().FindById(id)
}

// Delete deletes one scheduled document.
func (helper *ScheduledHelper) Delete(documentId string) error {
	return helper.store().Delete(documentId)
}

// Update merges a patch into one scheduled entry.
func (helper *ScheduledHelper) Update(documentId string, patch map[string]any) error {
	return helper.store().Patch(documentId, patch)
}

func (helper *ScheduledHelper) store() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Scheduled}
}
