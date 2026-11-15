type InvalidRequest struct {
	Message     string
	Field       string
	ErrorNumber int
}

func (InvalidRequest) EnvelopeRefused(reason string, number int) *InvalidRequest {
	return &InvalidRequest{Message: reason, Field: "envelope", ErrorNumber: number}
}

func (InvalidRequest) TypeRefused(reason string, number int) *InvalidRequest {
	return &InvalidRequest{Message: reason, Field: "type", ErrorNumber: number}
}

func (failure *InvalidRequest) Error() string {
	return "invalid request: " + failure.Field
}

func (failure *InvalidRequest) Number() int {
	return failure.ErrorNumber
}
