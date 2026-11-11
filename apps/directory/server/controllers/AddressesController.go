import (
	"cmp"
	"strings"

	"mesail.dev/packages/recipients"
)

// @RequestRoute("/api/v1/directory/addresses")
// @Allow(Service)
type AddressesController struct {
	// @Needs
	callers *CallerService
	// @Needs
	addresses *AddressService
}

// @GetRoute("/{address}")
func (controller *AddressesController) GetAddress(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	address string,
) (Address, error) {
	caller, refused := controller.callers.Caller(authorization)
	domainId := recipients.Addresses{}.Domain(strings.ToLower(address))
	return controller.findUnless(cmp.Or(refused, controller.callers.ServiceFor(caller, domainId)), address)
}

func (controller *AddressesController) findUnless(refusal error, address string) (Address, error) {
	if refusal != nil {
		return Address{}, refusal
	}
	return controller.addresses.Find(address)
}
