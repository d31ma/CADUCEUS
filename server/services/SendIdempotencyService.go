// SendIdempotencyService keeps the exactly-once acceptance journal of
// outbound mail.
// @Service
type SendIdempotencyService struct {
	// @Needs
	sendRequestsHelper *SendRequestsHelper
}

// Journal answers the request's acceptance journal.
func (service *SendIdempotencyService) Journal() SendIdempotency {
	return SendIdempotency{Requests: service.sendRequestsHelper}
}
