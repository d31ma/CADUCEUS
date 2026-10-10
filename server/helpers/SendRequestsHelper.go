import "strings"

// SendRequestsHelper keeps the idempotency records of send requests.
// @Helper
type SendRequestsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores one send request record and answers its document id.
func (helper *SendRequestsHelper) Put(record map[string]any) (string, error) {
	return helper.store().Put(record)
}

// Find answers the document id and record of one owner's idempotency key, or
// "" and nil. The query narrows by the trusted owner and the key is checked
// here, so one send is never mistaken for another request of the mailbox.
func (helper *SendRequestsHelper) Find(ownerEmail string, idempotencyKey string) (string, map[string]any, error) {
	documents, err := helper.store().Equal("ownerEmail", strings.ToLower(ownerEmail))
	if err != nil {
		return "", nil, err
	}
	for _, document := range documents {
		record := StoredRecords{}.Decoded(document.Data)
		if record["idempotencyKey"] == idempotencyKey {
			return document.Id, record, nil
		}
	}
	return "", nil, nil
}

// Update merges a patch into one send request record.
func (helper *SendRequestsHelper) Update(documentId string, patch map[string]any) error {
	return helper.store().Patch(documentId, patch)
}

// Delete deletes one send request record.
func (helper *SendRequestsHelper) Delete(documentId string) error {
	return helper.store().Delete(documentId)
}

func (helper *SendRequestsHelper) store() CollectionStore {
	return CollectionStore{Database: helper.fyloHelper.Database(), Collection: FyloCollections{}.Named().SendRequests}
}
