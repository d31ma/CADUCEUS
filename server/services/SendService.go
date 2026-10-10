// SendService accepts outbound mail (now, delayed with undo, or scheduled)
// and keeps its Sent copy.
// @Service
type SendService struct {
	// @Needs
	fyloHelper *FyloHelper
	// @Needs
	smtpService *SmtpService
}

// Sent answers the request's outbound mail acceptance.
func (service *SendService) Sent() SentMail {
	return DeliveryStores{Fylo: service.fyloHelper}.Sent(service.smtpService)
}
