import (
	"cmp"
	"slices"
)

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
	return service.names(slices.DeleteFunc(domains, service.inactive)), failure
}

func (service *TenantService) inactive(domain DomainDocument) bool {
	return domain.State != service.active()
}

func (service *TenantService) refusal(found bool, state string) error {
	switch {
	case !found, state == service.disabled(), state == service.removing():
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

func (service *TenantService) active() string {
	return "active"
}

func (service *TenantService) removing() string {
	return "removing"
}

func (service *TenantService) disabled() string {
	return "disabled"
}
