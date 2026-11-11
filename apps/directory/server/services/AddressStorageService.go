import "mesail.dev/packages/recipients"

// @Service
type AddressStorageService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (storage *AddressStorageService) Find(address string) (AddressDocument, bool, error) {
	route := AddressDocument{}
	collection := storage.names.Addresses(recipients.Addresses{}.Domain(address))
	found, failure := storage.documents.FindOne(collection, map[string]any{storage.addressField(): address}, &route)
	return route, found, failure
}

func (storage *AddressStorageService) Insert(address AddressDocument) error {
	_, failure := storage.documents.Insert(storage.names.Addresses(address.DomainId), address)
	return failure
}

func (storage *AddressStorageService) addressField() string {
	return "address"
}

func (storage *AddressStorageService) InsertIfAbsent(address AddressDocument) error {
	_, found, failure := storage.Find(address.Address)
	return storage.insertUnless(failure, found, address)
}

func (storage *AddressStorageService) insertUnless(failure error, found bool, address AddressDocument) error {
	switch {
	case failure != nil:
		return failure
	case found:
		return nil
	}
	return storage.Insert(address)
}
