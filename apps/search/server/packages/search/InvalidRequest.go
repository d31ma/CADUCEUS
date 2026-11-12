package search

type InvalidRequest struct {
	Message     string
	Field       string
	ErrorNumber int
}

func (InvalidRequest) QueryTooLong() InvalidRequest {
	return InvalidRequest{Message: "the search text is too long", Field: "q", ErrorNumber: 4001}
}

func (InvalidRequest) NotADate(field string) InvalidRequest {
	return InvalidRequest{Message: field + " must be a date or an RFC 3339 time", Field: field, ErrorNumber: 4002}
}

func (InvalidRequest) NotTrueOrFalse(field string) InvalidRequest {
	return InvalidRequest{Message: field + " must be true or false", Field: field, ErrorNumber: 4003}
}

func (InvalidRequest) BadCursor(message string) InvalidRequest {
	return InvalidRequest{Message: message, Field: "cursor", ErrorNumber: 4004}
}

func (InvalidRequest) BadLimit() InvalidRequest {
	return InvalidRequest{Message: "limit must be a number from 1 to 100", Field: "limit", ErrorNumber: 4005}
}

func (InvalidRequest) NotATime(field string) InvalidRequest {
	return InvalidRequest{Message: field + " is not an RFC 3339 time", Field: field, ErrorNumber: 4006}
}

func (InvalidRequest) MissingIds() InvalidRequest {
	return InvalidRequest{Message: "messageId and conversationId are required", Field: "payload", ErrorNumber: 4007}
}

func (InvalidRequest) PayloadMismatch() InvalidRequest {
	return InvalidRequest{Message: "the event payload does not match its type", Field: "payload", ErrorNumber: 4008}
}

func (InvalidRequest) NotWritable() InvalidRequest {
	return InvalidRequest{Message: "the value cannot be written as JSON", Field: "body", ErrorNumber: 4009}
}

func (InvalidRequest) MissingFields() InvalidRequest {
	return InvalidRequest{Message: "the value does not have the expected fields", Field: "body", ErrorNumber: 4010}
}

func (InvalidRequest) BadEnvelope(message string) InvalidRequest {
	return InvalidRequest{Message: message, Field: "envelope", ErrorNumber: 4011}
}

func (failure InvalidRequest) Error() string {
	return failure.Message
}
