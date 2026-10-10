import "net/http"

// InboundWebhookRoute answers POST /inbound/webhook: the canonical signed
// inbound endpoint. The JSON body must carry an X-Caduceus-Signature
// HMAC-SHA256 of its exact bytes made with INBOUND_WEBHOOK_SECRET.
type InboundWebhookRoute struct {
	Inbound *InboundDeliveryService
}

// Deliver answers POST /inbound/webhook.
func (route InboundWebhookRoute) Deliver(request *http.Request) *http.Response {
	value, refusal := (SystemSignature{}).Body(request, "INBOUND_WEBHOOK_SECRET")
	if refusal != nil {
		return refusal
	}
	body, _ := value.(map[string]any)
	if !(RecordValues{}).Truthy(body["recipient"]) || !(RecordValues{}).Truthy(body["sender"]) {
		return Answer{}.BadRequest("recipient and sender required")
	}
	return DeliveryReply{}.Settled(route.Inbound.Delivery().Deliver(body))
}
