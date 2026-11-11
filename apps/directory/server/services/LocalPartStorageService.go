import (
	"slices"

	"mesail.dev/packages/addresses"
)

// @Service
type LocalPartStorageService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (storage *LocalPartStorageService) Find(localPart string) (LocalPartDocument, error) {
	document := LocalPartDocument{}
	_, failure := storage.documents.FindOne(storage.names.LocalParts(), map[string]any{storage.localPartField(): localPart}, &document)
	return document, failure
}

func (storage *LocalPartStorageService) ForAddressesIn(domainId string) ([]LocalPartDocument, error) {
	documents := []LocalPartDocument{}
	failure := storage.documents.All(storage.names.LocalParts(), map[string]any{storage.ownerField(): addresses.Owners{}.Addresses()}, &documents)
	return storage.reservedIn(documents, domainId), failure
}

func (storage *LocalPartStorageService) Delete(document LocalPartDocument) error {
	return storage.documents.Delete(storage.names.LocalParts(), document.Id)
}

func (storage *LocalPartStorageService) Store(document LocalPartDocument) error {
	document.DomainIds = append([]string{}, document.DomainIds...)
	return storage.insertOrReplace(document)
}

func (storage *LocalPartStorageService) insertOrReplace(document LocalPartDocument) error {
	if document.Id == "" {
		_, failure := storage.documents.Insert(storage.names.LocalParts(), document)
		return failure
	}
	id := document.Id
	document.Id = ""
	return storage.documents.Replace(storage.names.LocalParts(), id, document)
}

func (storage *LocalPartStorageService) reservedIn(documents []LocalPartDocument, domainId string) []LocalPartDocument {
	return slices.DeleteFunc(documents, func(document LocalPartDocument) bool { return !slices.Contains(document.DomainIds, domainId) })
}

func (storage *LocalPartStorageService) ownerField() string {
	return "owner"
}

func (storage *LocalPartStorageService) localPartField() string {
	return "localPart"
}
