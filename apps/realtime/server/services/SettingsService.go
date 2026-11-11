// @Service
type SettingsService struct{}

func (service *SettingsService) Gateway() GatewaySettings {
	return GatewaySettings{}
}
