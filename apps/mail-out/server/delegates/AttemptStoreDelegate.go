import "errors"

// @Delegate
// @NeedsAll()
type AttemptStoreDelegate struct {
	dataSource DataSource
}

func (AttemptStoreDelegate) collection() string { return "delivery-attempts" }

func (AttemptStoreDelegate) pageSize() int { return 100 }

func (store *AttemptStoreDelegate) Record(attempt DeliveryAttempt) error {
	collection, err := store.ready(attempt.DomainId)
	_, insertErr := store.dataSource.Insert(collection, attempt)
	return errors.Join(err, insertErr)
}

func (store *AttemptStoreDelegate) forItem(domainId string, itemId string) ([]DeliveryAttempt, error) {
	collection, err := store.ready(domainId)
	window, scrollErr := store.dataSource.Scroll(collection, map[string]any{"itemId": itemId}, "", store.pageSize())
	attempts := []DeliveryAttempt{}
	decodeErr := window.Decode(&attempts)
	return attempts, errors.Join(err, scrollErr, decodeErr)
}

func (store *AttemptStoreDelegate) ready(domainId string) (string, error) {
	collection := StorageUtils{}.CollectionName(store.collection(), domainId)
	return collection, store.dataSource.CreateCollection(collection)
}
