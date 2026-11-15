import (
	"cmp"
	"strings"

	"mesail.dev/packages/recipients"
)

// @Service
type AddressRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	addresses *AddressService
}

func (service *AddressRouteService) Find(authorization string, address string) (Address, error) {
	caller, refused := service.callers.Caller(authorization)
	domainId := recipients.Addresses{}.Domain(strings.ToLower(address))
	return service.findUnless(cmp.Or(refused, service.callers.ServiceFor(caller, domainId)), address)
}

func (service *AddressRouteService) findUnless(refusal error, address string) (Address, error) {
	if refusal != nil {
		return Address{}, refusal
	}
	return service.addresses.Find(address)
}
