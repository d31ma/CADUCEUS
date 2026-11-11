import "errors"

// @Delegate
// @NeedsAll()
type SuppressionStoreDelegate struct {
	dataSource DataSource
}

func (SuppressionStoreDelegate) collection() string { return "suppressions" }

func (SuppressionStoreDelegate) oneDocument() int { return 1 }

func (store *SuppressionStoreDelegate) Contains(domainId string, address string) (bool, error) {
	collection, err := store.ready(domainId)
	window, scrollErr := store.dataSource.Scroll(collection, store.byAddress(address), "", store.oneDocument())
	found := []Suppression{}
	decodeErr := window.Decode(&found)
	return len(found) > 0, errors.Join(err, scrollErr, decodeErr)
}

func (store *SuppressionStoreDelegate) Add(suppression Suppression) error {
	suppression.AddressHash = StorageUtils{}.AddressHash(suppression.Address)
	known, err := store.Contains(suppression.DomainId, suppression.Address)
	return store.insert(suppression, known, err)
}

func (store *SuppressionStoreDelegate) ready(domainId string) (string, error) {
	collection := StorageUtils{}.CollectionName(store.collection(), domainId)
	return collection, store.dataSource.CreateCollection(collection)
}

func (SuppressionStoreDelegate) byAddress(address string) map[string]any {
	return map[string]any{"addressHash": StorageUtils{}.AddressHash(address)}
}

func (store *SuppressionStoreDelegate) insert(suppression Suppression, known bool, failure error) error {
	if known || failure != nil {
		return failure
	}
	_, insertErr := store.dataSource.Insert(StorageUtils{}.CollectionName(store.collection(), suppression.DomainId), suppression)
	return insertErr
}
