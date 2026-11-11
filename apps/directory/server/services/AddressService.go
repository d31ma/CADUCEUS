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
	_, hosted, domainFailure := service.domains.Find(domainId)
	route, found, failure := service.route(hosted, domainId, address)
	wire := Address{Address: route.Address, DomainId: route.DomainId, Kind: route.Kind, TargetId: route.TargetId}
	return wire, cmp.Or(domainFailure, failure, service.missing(invalid == nil, found))
}

func (service *AddressService) route(hosted bool, domainId string, address string) (AddressDocument, bool, error) {
	if !hosted {
		return AddressDocument{}, false, nil
	}
	return service.addresses.Find(address)
}

func (service *AddressService) missing(valid bool, found bool) error {
	switch {
	case !valid, !found:
		return &NotFound{Message: "no such address"}
	}
	return nil
}
