// @Service
type FanOutService struct {
	// @Needs
	connectionStoreService *ConnectionStoreService
	// @Needs
	gatewayPushService *GatewayPushService
}

func (service *FanOutService) ToMembers(domainId string, memberIds []string, event RealtimeEvent) error {
	return nil
}

func (service *FanOutService) ToMembersExcept(domainId string, memberIds []string, exceptMemberId string, event RealtimeEvent) error {
	return nil
}

func (service *FanOutService) ToDomain(domainId string, event RealtimeEvent) error {
	return nil
}

func (service *FanOutService) ToConnection(domainId string, connectionId string, event RealtimeEvent) error {
	return nil
}
