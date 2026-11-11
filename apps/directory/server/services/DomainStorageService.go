// @Service
type DomainStorageService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (storage *DomainStorageService) All() ([]DomainDocument, error) {
	domains := []DomainDocument{}
	failure := storage.documents.All(storage.names.Domains(), map[string]any{}, &domains)
	return domains, failure
}

func (storage *DomainStorageService) Removing() ([]DomainDocument, error) {
	domains := []DomainDocument{}
	failure := storage.documents.All(storage.names.Domains(), map[string]any{storage.stateField(): storage.removing()}, &domains)
	return domains, failure
}

func (storage *DomainStorageService) RemovingIds() ([]string, error) {
	domains, failure := storage.Removing()
	return storage.ids(domains), failure
}

func (storage *DomainStorageService) Find(domainId string) (DomainDocument, bool, error) {
	domain := DomainDocument{}
	found, failure := storage.documents.FindOne(storage.names.Domains(), map[string]any{storage.domainField(): domainId}, &domain)
	return domain, found, failure
}

func (storage *DomainStorageService) Insert(domain DomainDocument) error {
	_, failure := storage.documents.Insert(storage.names.Domains(), domain)
	return failure
}

func (storage *DomainStorageService) Save(domain DomainDocument) error {
	id := domain.Id
	domain.Id = ""
	return storage.documents.Replace(storage.names.Domains(), id, domain)
}

func (storage *DomainStorageService) Delete(domain DomainDocument) error {
	return storage.documents.Delete(storage.names.Domains(), domain.Id)
}

func (storage *DomainStorageService) ids(domains []DomainDocument) []string {
	ids := []string{}
	for _, domain := range domains {
		ids = append(ids, domain.DomainId)
	}
	return ids
}

func (storage *DomainStorageService) stateField() string {
	return "state"
}

func (storage *DomainStorageService) removing() string {
	return "removing"
}

func (storage *DomainStorageService) domainField() string {
	return "domainId"
}
