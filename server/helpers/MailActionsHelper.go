// MailActionsHelper reads and writes the journal of mail actions.
// @Helper
type MailActionsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores a journal entry and answers its document id.
func (helper *MailActionsHelper) Put(entry map[string]any) (string, error) {
	return helper.actions().Put(entry)
}

// FindById answers the document id and entry of one action that belongs to
// the owner ("" for any owner), or "" and nil. Historical FYLO roots may hold
// a logical id distinct from the document id, so that is looked up too.
func (helper *MailActionsHelper) FindById(id string, ownerEmail string) (string, map[string]any, error) {
	if id == "" {
		return "", nil, nil
	}
	entries := MailActionEntries{}
	documents, err := helper.fyloHelper.Database().GetDoc(FyloCollections{}.Named().MailActions, id)
	entry, err := entries.Fetched(documents, err, id)
	if err != nil {
		return "", nil, err
	}
	if entries.Owned(entry, ownerEmail) {
		return id, entry.(map[string]any), nil
	}
	matches, err := helper.actions().Where("id", id)
	record := entries.Matching(matches, id, ownerEmail)
	if err != nil || record.Id == "" {
		return "", nil, err
	}
	return record.Id, record.Data, nil
}

// ListForOwner answers the owner's journal entries.
func (helper *MailActionsHelper) ListForOwner(ownerEmail string) ([]FyloRecord, error) {
	records, err := helper.actions().Where("ownerEmail", ownerEmail)
	return MailActionEntries{}.OwnedBy(records, ownerEmail), err
}

// FindByClientRequestId answers the owner's entry made by one client
// request, or "" and nil.
func (helper *MailActionsHelper) FindByClientRequestId(ownerEmail string, clientRequestId string) (string, map[string]any, error) {
	records, err := helper.ListForOwner(ownerEmail)
	record := MailActionEntries{}.ForClientRequest(records, clientRequestId)
	return record.Id, record.Data, err
}

// Update merges fields into one journal entry.
func (helper *MailActionsHelper) Update(documentId string, patch map[string]any) error {
	return helper.actions().Patch(documentId, patch)
}

// UpdateById finds the latest document of an action before merging fields
// into it, and answers false when the owner has no such action.
func (helper *MailActionsHelper) UpdateById(id string, patch map[string]any, ownerEmail string) (bool, error) {
	documentId, record, err := helper.FindById(id, ownerEmail)
	if err != nil || documentId == "" || record == nil {
		return false, err
	}
	return true, helper.Update(documentId, patch)
}

func (helper *MailActionsHelper) actions() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().MailActions}
}
