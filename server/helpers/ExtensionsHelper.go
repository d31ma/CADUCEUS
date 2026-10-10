import "strings"

// ExtensionsHelper keeps the extensions each mailbox installed.
// @Helper
type ExtensionsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores one extension record and answers its document id.
func (helper *ExtensionsHelper) Put(record map[string]any) (string, error) {
	return helper.store().Put(record)
}

// ListForOwner answers the extension records of one owner as
// {docId, ...record}.
func (helper *ExtensionsHelper) ListForOwner(owner string) ([]map[string]any, error) {
	return StoredRecords{}.Where(helper.store(), "owner", strings.ToLower(owner))
}

// FindForOwner answers the owner's record of one extension, or nil.
func (helper *ExtensionsHelper) FindForOwner(owner string, extensionId string) (map[string]any, error) {
	records, err := helper.ListForOwner(owner)
	for _, record := range records {
		if record["extensionId"] == extensionId {
			return record, nil
		}
	}
	return nil, err
}

// Update merges a patch into one extension record.
func (helper *ExtensionsHelper) Update(documentId string, patch map[string]any) error {
	return helper.store().Patch(documentId, patch)
}

// Delete deletes one extension record.
func (helper *ExtensionsHelper) Delete(documentId string) error {
	return helper.store().Delete(documentId)
}

func (helper *ExtensionsHelper) store() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Extensions}
}
