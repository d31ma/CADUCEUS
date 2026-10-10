import "net/http"

// @RequestRoute("/inbound/webhook")
// @Public
type InboundWebhookController struct {
	// @Needs
	inboundDeliveryService *InboundDeliveryService
}

// @PostRoute
func (controller *InboundWebhookController) PostInbound(request *http.Request) *http.Response {
	return InboundWebhookRoute{Inbound: controller.inboundDeliveryService}.Deliver(request)
}
