// @Service
type DomainRegistryService struct {
	// @Needs
	dataSource DataSource
}

func (service *DomainRegistryService) Remember(domainId string) error {
	return nil
}

func (service *DomainRegistryService) List(cursor string, limit int) ([]string, string, error) {
	return []string{}, "", nil
}
