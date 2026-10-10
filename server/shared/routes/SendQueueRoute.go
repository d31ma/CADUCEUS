import "net/http"

// SendQueueRoute answers the signed system passes over the outbound queue:
// /send/tick (the delivery worker's interval pass) and /send/flush (manual
// recovery, with queue health before and after).
type SendQueueRoute struct {
	Queue *OutboundQueueService
}

// Tick answers POST /send/tick.
func (route SendQueueRoute) Tick(request *http.Request) *http.Response {
	if _, refusal := (SystemSignature{}).Body(request, "INBOUND_WEBHOOK_SECRET"); refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.pass(false))
}

// Flush answers POST /send/flush.
func (route SendQueueRoute) Flush(request *http.Request) *http.Response {
	if _, refusal := (SystemSignature{}).Body(request, "INBOUND_WEBHOOK_SECRET"); refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.pass(true))
}

func (route SendQueueRoute) pass(flush bool) (any, error) {
	queue := route.Queue.Queue()
	before, err := queue.Status(EpochTime{}.Now())
	if err != nil {
		return nil, err
	}
	result, err := queue.ProcessDue(EpochTime{}.Now())
	if err != nil {
		return nil, err
	}
	after, err := queue.Status(EpochTime{}.Now())
	if err != nil {
		return nil, err
	}
	result["queue"] = after
	if flush {
		result["flushed"], result["before"] = result["delivered"], before
	}
	return result, nil
}
