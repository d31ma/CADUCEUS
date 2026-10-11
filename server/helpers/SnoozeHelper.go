// SnoozeHelper reads and writes snoozed conversations.
// @Helper
type SnoozeHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores a snooze entry and answers its document id.
func (helper *SnoozeHelper) Put(entry map[string]any) (string, error) {
	return helper.snoozed().Put(entry)
}

// listAll answers every snooze entry.
func (helper *SnoozeHelper) listAll() ([]FyloRecord, error) {
	return helper.snoozed().All()
}

// ListActive answers the active entries the owner may see ("" for every
// mailbox).
func (helper *SnoozeHelper) ListActive(ownerEmail string) ([]FyloRecord, error) {
	entries, err := helper.listAll()
	return SnoozeRecords{}.Active(entries, ownerEmail), err
}

// ListDue answers the active entries due by now (milliseconds), soonest
// first, at most limit (1 to 500) of them.
func (helper *SnoozeHelper) ListDue(now int64, limit int) ([]FyloRecord, error) {
	active, err := helper.ListActive("")
	return SnoozeRecords{}.Due(active, now, limit), err
}

// FindById answers the document id and entry with a logical id, or "" and nil.
func (helper *SnoozeHelper) FindById(id string) (string, map[string]any, error) {
	return helper.snoozed().FindById(id)
}

// FindActiveForThread answers the owner's active entry for a thread, or ""
// and nil.
func (helper *SnoozeHelper) FindActiveForThread(ownerEmail string, threadId string) (string, map[string]any, error) {
	entries, err := helper.ListActive(ownerEmail)
	record := SnoozeRecords{}.ForThread(entries, threadId)
	return record.Id, record.Data, err
}

// FindByClientRequestId answers the owner's entry made by one client
// request, or "" and nil.
func (helper *SnoozeHelper) FindByClientRequestId(ownerEmail string, clientRequestId string) (string, map[string]any, error) {
	entries, err := helper.listAll()
	record := SnoozeRecords{}.ForClientRequest(entries, ownerEmail, clientRequestId)
	return record.Id, record.Data, err
}

// Update merges fields into one snooze entry.
func (helper *SnoozeHelper) Update(documentId string, patch map[string]any) error {
	return helper.snoozed().Patch(documentId, patch)
}

func (helper *SnoozeHelper) snoozed() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Snoozed}
}
