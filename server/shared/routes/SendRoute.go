import "net/http"

// SendRoute answers POST /send: accept one outbound message for delivery.
type SendRoute struct {
	Access      *AccessHelper
	Send        *SendService
	Idempotency *SendIdempotencyService
	Limits      *RateLimitService
}

// Create answers POST /send.
func (route SendRoute) Create(request *http.Request) *http.Response {
	caller, body, refusal := (DeliveryReply{}).Caller(route.Access, request)
	if refusal != nil {
		return refusal
	}
	acceptance := route.Send.Acceptance(route.Idempotency.Journal(), route.Limits)
	address := acceptance.IpAddress(request.Header.Get("X-Forwarded-For"), request.Header.Get("X-Real-IP"))
	return DeliveryReply{}.Settled(acceptance.Accept(caller, body, address))
}
