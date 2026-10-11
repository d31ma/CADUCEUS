// OutboundQueueService keeps the durable outbound queue: delivery passes,
// retries with backoff, bounces and queue health.
// @Service
type OutboundQueueService struct {
	// @Needs
	fyloHelper *FyloHelper
	// @Needs
	smtpService *SmtpService
}

// Queue answers the request's outbound queue.
func (service *OutboundQueueService) Queue() OutboundQueue {
	return DeliveryStores{Fylo: service.fyloHelper}.Queue(service.smtpService)
}
