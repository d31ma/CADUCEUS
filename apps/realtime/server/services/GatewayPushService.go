// @Service
type GatewayPushService struct {
	// @Needs
	settingsService *SettingsService
}

func (service *GatewayPushService) Push(connectionId string, event RealtimeEvent) error {
	return nil
}

func (service *GatewayPushService) Close(connectionId string, code int) error {
	return nil
}
