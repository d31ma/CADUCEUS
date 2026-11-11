// @Service
type SessionService struct {
	// @Needs
	tokenCheckService *TokenCheckService
	// @Needs
	ticketService *TicketService
	// @Needs
	connectionStoreService *ConnectionStoreService
	// @Needs
	domainRegistryService *DomainRegistryService
	// @Needs
	presenceService *PresenceService
}

func (service *SessionService) Authenticate(request GatewayMessage, message RealtimeClientMessage) (GatewayReply, error) {
	return GatewayReply{}, &Unauthenticated{}
}
