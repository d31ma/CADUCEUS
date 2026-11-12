package mailout

type InvalidSendRequest struct {
	Reason      string
	ErrorNumber int
}

func (InvalidSendRequest) Body(reason string) InvalidSendRequest {
	return InvalidSendRequest{Reason: "the event envelope is not valid: " + reason, ErrorNumber: 3004}
}

func (request InvalidSendRequest) Producer() InvalidSendRequest {
	return request.field("producer", 3005)
}

func (request InvalidSendRequest) Payload() InvalidSendRequest {
	return request.field("payload", 3006)
}

func (request InvalidSendRequest) MessageId() InvalidSendRequest {
	return request.field("messageId", 3007)
}

func (request InvalidSendRequest) IdempotencyKey() InvalidSendRequest {
	return request.field("idempotencyKey", 3008)
}

func (request InvalidSendRequest) Rcpt() InvalidSendRequest {
	return request.field("rcpt", 3009)
}

func (request InvalidSendRequest) From() InvalidSendRequest {
	return request.field("from", 3010)
}

func (request InvalidSendRequest) Error() string {
	return request.Reason
}

func (request InvalidSendRequest) Number() int {
	return request.ErrorNumber
}

func (InvalidSendRequest) field(name string, number int) InvalidSendRequest {
	return InvalidSendRequest{Reason: "the email.send.requested field " + name + " is missing or not valid", ErrorNumber: number}
}
