type InvalidRequest struct {
	Message     string
	Field       string
	ErrorNumber int
}

func (InvalidRequest) EnvelopeRefused(reason string) *InvalidRequest {
	return &InvalidRequest{Message: reason, Field: "envelope", ErrorNumber: 5009}
}

func (InvalidRequest) TypeRefused(reason string) *InvalidRequest {
	return &InvalidRequest{Message: reason, Field: "type", ErrorNumber: 5010}
}

func (failure *InvalidRequest) Error() string {
	return "invalid request: " + failure.Field
}

func (failure *InvalidRequest) Number() int {
	return failure.ErrorNumber
}
