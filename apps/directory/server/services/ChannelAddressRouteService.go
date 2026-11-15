// @Service
type ChannelAddressRouteService struct {
	// @Needs
	admission *AdmissionService
	// @Needs
	channels *ChannelAddressService
}

func (service *ChannelAddressRouteService) Add(authorization string, request ChannelAddressRequest) (Address, error) {
	return service.channels.Add(service.admission.Admit(authorization), request)
}

func (service *ChannelAddressRouteService) Remove(authorization string, address string) error {
	return service.channels.Remove(service.admission.Admit(authorization), address)
}
