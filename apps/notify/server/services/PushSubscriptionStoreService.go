import "cmp"

// @Service
type PushSubscriptionStoreService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (store *PushSubscriptionStoreService) Upsert(record PushSubscriptionRecord) error {
	patch := PushKeysPatch{MemberId: record.MemberId, P256dh: record.P256dh, Auth: record.Auth}
	return store.documents.Upsert(store.names.PushSubscriptions(record.DomainId), map[string]any{store.endpointField(): record.Endpoint}, record, patch)
}

func (store *PushSubscriptionStoreService) DeleteForMember(domainId string, memberId string, endpoint string) error {
	collection := store.names.PushSubscriptions(domainId)
	found := []PushSubscriptionRecord{}
	failure := store.documents.Matching(collection, map[string]any{store.endpointField(): endpoint, store.memberField(): memberId}, store.listLimit(), &found)
	return store.deleteUnless(failure, collection, found)
}

func (store *PushSubscriptionStoreService) ListForMember(domainId string, memberId string) ([]PushSubscriptionRecord, error) {
	found := []PushSubscriptionRecord{}
	failure := store.documents.Matching(store.names.PushSubscriptions(domainId), map[string]any{store.memberField(): memberId}, store.listLimit(), &found)
	return found, failure
}

func (store *PushSubscriptionStoreService) Delete(domainId string, id string) error {
	return store.documents.Delete(store.names.PushSubscriptions(domainId), id)
}

func (store *PushSubscriptionStoreService) deleteUnless(failure error, collection string, found []PushSubscriptionRecord) error {
	if failure != nil {
		return failure
	}
	return store.deleteAll(collection, found)
}

func (store *PushSubscriptionStoreService) deleteAll(collection string, found []PushSubscriptionRecord) error {
	failures := []error{}
	for _, record := range found {
		failures = append(failures, store.documents.Delete(collection, record.Id))
	}
	return cmp.Or(failures...)
}

func (store *PushSubscriptionStoreService) endpointField() string {
	return "endpoint"
}

func (store *PushSubscriptionStoreService) memberField() string {
	return "memberId"
}

func (store *PushSubscriptionStoreService) listLimit() int {
	return 100
}
