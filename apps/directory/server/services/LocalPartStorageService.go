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

func (storage *LocalPartStorageService) localPartField() string {
	return "localPart"
}
