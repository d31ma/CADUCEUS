// InboundService applies domain routes and inbox rules to inbound mail.
// @Service
type InboundService struct {
	// @Needs
	fyloHelper *FyloHelper
	// @Needs
	smtpService *SmtpService
}

// Routing answers the request's route and inbox-rule handling.
func (service *InboundService) Routing() InboundRouting {
	return DeliveryStores{Fylo: service.fyloHelper}.Routing(service.smtpService)
}
