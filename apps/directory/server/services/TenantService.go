import "cmp"

// @Service
type TenantService struct {
	// @Needs
	domains *DomainStorageService
}

func (service *TenantService) Require(domainId string) (DomainDocument, error) {
	domain, found, failure := service.domains.Find(domainId)
	return domain, cmp.Or(failure, service.refusal(found, domain.State))
}

func (service *TenantService) Hosted() (map[string]bool, error) {
	domains, failure := service.domains.All()
	return service.names(domains), failure
}

func (service *TenantService) refusal(found bool, state string) error {
	switch {
	case !found, state == service.disabled():
		return &Forbidden{Message: "this domain is not hosted here"}
	}
	return nil
}

func (service *TenantService) names(domains []DomainDocument) map[string]bool {
	hosted := map[string]bool{}
	for _, domain := range domains {
		hosted[domain.DomainId] = true
	}
	return hosted
}

func (service *TenantService) disabled() string {
	return "disabled"
}
