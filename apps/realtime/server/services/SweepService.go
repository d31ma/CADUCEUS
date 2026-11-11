// @Service
type SweepService struct {
	// @Needs
	domainRegistryService *DomainRegistryService
	// @Needs
	connectionStoreService *ConnectionStoreService
	// @Needs
	gatewayPushService *GatewayPushService
	// @Needs
	presenceService *PresenceService
}

func (service *SweepService) Run(budget int) (SweepReport, error) {
	return SweepReport{}, nil
}
