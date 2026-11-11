// @RequestRoute("/api/v1/directory/channel-addresses")
// @Allow(DomainAdmin)
type ChannelAddressesController struct {
	// @Needs
	admission *AdmissionService
	// @Needs
	channels *ChannelAddressService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *ChannelAddressesController) PostChannelAddress(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request ChannelAddressRequest,
) (Address, error) {
	return controller.channels.Add(controller.admission.Admit(authorization), request)
}

// @DeleteRoute("/{address}")
// @ResponseStatus(204)
func (controller *ChannelAddressesController) DeleteChannelAddress(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	address string,
) error {
	return controller.channels.Remove(controller.admission.Admit(authorization), address)
}
