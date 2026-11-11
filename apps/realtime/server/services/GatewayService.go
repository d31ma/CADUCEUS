// @Service
type GatewayService struct {
	// @Needs
	sessionService *SessionService
	// @Needs
	typingService *TypingService
	// @Needs
	presenceService *PresenceService
	// @Needs
	connectionStoreService *ConnectionStoreService
	// @Needs
	gatewayPushService *GatewayPushService
}

func (service *GatewayService) Connect(request GatewayConnect) error {
	return nil
}

func (service *GatewayService) Message(request GatewayMessage) (GatewayReply, error) {
	return GatewayReply{}, nil
}

func (service *GatewayService) Disconnect(request GatewayDisconnect) error {
	return nil
}
