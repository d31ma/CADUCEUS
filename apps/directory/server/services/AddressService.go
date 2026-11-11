import (
	"cmp"

	"mesail.dev/packages/recipients"
)

// @Service
type AddressService struct {
	// @Needs
	domains *DomainStorageService
	// @Needs
	addresses *AddressStorageService
}

func (service *AddressService) Find(raw string) (Address, error) {
	address, invalid := recipients.Addresses{}.Normalize(raw)
	domainId := recipients.Addresses{}.Domain(address)
	domain, hosted, domainFailure := service.domains.Find(domainId)
	active := service.active(hosted, domain.State)
	route, found, failure := service.route(active, address)
	wire := Address{Address: route.Address, DomainId: route.DomainId, Kind: route.Kind, TargetId: route.TargetId}
	return wire, cmp.Or(domainFailure, failure, service.missing(invalid == nil), service.notHosted(active), service.missing(found))
}

func (service *AddressService) active(hosted bool, state string) bool {
	return hosted && state == service.activeState()
}

func (service *AddressService) route(active bool, address string) (AddressDocument, bool, error) {
	if !active {
		return AddressDocument{}, false, nil
	}
	return service.addresses.Find(address)
}

func (service *AddressService) notHosted(active bool) error {
	if active {
		return nil
	}
	return &DomainNotHosted{Message: "this domain is not hosted here"}
}

func (service *AddressService) missing(found bool) error {
	if found {
		return nil
	}
	return &NotFound{Message: "no such address"}
}

func (service *AddressService) activeState() string {
	return "active"
}
