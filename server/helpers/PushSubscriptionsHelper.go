import "strings"

// PushSubscriptionsHelper keeps the web push subscriptions of each user.
// @Helper
type PushSubscriptionsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// ListForUser answers the subscriptions of one user as {docId, ...record}.
func (helper *PushSubscriptionsHelper) ListForUser(userEmail string) ([]map[string]any, error) {
	return StoredRecords{}.Where(helper.store(), helper.userField(), strings.ToLower(userEmail))
}

// ListForAddress answers the subscriptions of the user with the address.
func (helper *PushSubscriptionsHelper) ListForAddress(address string) ([]map[string]any, error) {
	return helper.ListForUser(strings.ToLower(address))
}

// FindById answers the document id and subscription with the id, or "" and
// nil.
func (helper *PushSubscriptionsHelper) FindById(id string) (string, map[string]any, error) {
	return helper.store().FindById(id)
}

// Upsert stores the subscription under the id of its endpoint, keeping the
// creation time of an earlier one, and answers the stored record.
func (helper *PushSubscriptionsHelper) Upsert(subscription map[string]any) (map[string]any, error) {
	now := EpochTime{}.NowText()
	id := PushSubscriptionsRecords{}.SubscriptionId(RecordValues{}.Text(subscription["endpoint"]))
	documentId, existing, err := helper.FindById(id)
	if err != nil {
		return nil, err
	}
	record := StoredRecords{}.Without(subscription)
	record[helper.userField()] = strings.ToLower(RecordValues{}.Text(subscription[helper.userField()]))
	record[helper.idField()] = id
	record["createdAt"] = RecordValues{}.Coalesce(existing["createdAt"], now)
	record["updatedAt"] = now
	if documentId != "" {
		return record, helper.store().Patch(documentId, record)
	}
	_, err = helper.store().Put(record)
	return record, err
}

// DeleteByEndpoint deletes the subscription to an endpoint and reports
// whether there was one.
func (helper *PushSubscriptionsHelper) DeleteByEndpoint(endpoint string) (bool, error) {
	documentId, _, err := helper.FindById(PushSubscriptionsRecords{}.SubscriptionId(endpoint))
	if err != nil || documentId == "" {
		return false, err
	}
	return true, helper.store().Delete(documentId)
}

// DeleteDocument deletes one subscription document (the former deleteDoc).
func (helper *PushSubscriptionsHelper) DeleteDocument(documentId string) error {
	return helper.store().Delete(documentId)
}

func (PushSubscriptionsHelper) idField() string { return "id" }

func (PushSubscriptionsHelper) userField() string { return "userEmail" }

func (helper *PushSubscriptionsHelper) store() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().PushSubscriptions}
}
