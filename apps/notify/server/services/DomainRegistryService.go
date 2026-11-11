// @Service
type DomainRegistryService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (registry *DomainRegistryService) Register(domainId string) error {
	_, failure := registry.documents.InsertIfAbsent(registry.names.Domains(), map[string]any{"domainId": domainId}, DomainRecord{DomainId: domainId})
	return failure
}

func (registry *DomainRegistryService) All(limit int) ([]string, error) {
	records := []DomainRecord{}
	failure := registry.documents.Matching(registry.names.Domains(), map[string]any{}, limit, &records)
	return registry.domainIds(records), failure
}

func (registry *DomainRegistryService) domainIds(records []DomainRecord) []string {
	domainIds := []string{}
	for _, record := range records {
		domainIds = append(domainIds, record.DomainId)
	}
	return domainIds
}
