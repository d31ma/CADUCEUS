// @RequestRoute("/api/v1/directory/addresses")
// @Allow(Service)
type AddressesController struct {
	// @Needs
	addresses *AddressRouteService
}

// @GetRoute("/{address}")
func (controller *AddressesController) GetAddress(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	address string,
) (Address, error) {
	return controller.addresses.Find(authorization, address)
}
