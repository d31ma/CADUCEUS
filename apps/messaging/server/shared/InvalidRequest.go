type InvalidRequest struct {
	Message     string
	Field       string
	ErrorNumber int
}

func (InvalidRequest) BadLimit() *InvalidRequest {
	return &InvalidRequest{Message: "limit must be a whole number from 1 to 100", Field: "limit", ErrorNumber: 1506}
}

func (InvalidRequest) BadEnvelope(message string) *InvalidRequest {
	return &InvalidRequest{Message: message, ErrorNumber: 1508}
}

func (InvalidRequest) NotSubscribed(message string) *InvalidRequest {
	return &InvalidRequest{Message: message, ErrorNumber: 1509}
}

func (InvalidRequest) UnknownEventType(message string) *InvalidRequest {
	return &InvalidRequest{Message: message, ErrorNumber: 1510}
}

func (InvalidRequest) UnhandledEvent(message string) *InvalidRequest {
	return &InvalidRequest{Message: message, ErrorNumber: 1511}
}

func (failure *InvalidRequest) Error() string {
	return failure.Message
}

func (failure *InvalidRequest) Number() int {
	return failure.ErrorNumber
}
