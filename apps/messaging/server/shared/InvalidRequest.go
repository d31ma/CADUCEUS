type InvalidRequest struct {
	Message     string
	Field       string
	ErrorNumber int
}

func (InvalidRequest) BadLimit() *InvalidRequest {
	return &InvalidRequest{Message: "limit must be a whole number from 1 to 100", Field: "limit", ErrorNumber: 1506}
}

func (InvalidRequest) Kit(message string, number int) *InvalidRequest {
	return &InvalidRequest{Message: message, ErrorNumber: number}
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
