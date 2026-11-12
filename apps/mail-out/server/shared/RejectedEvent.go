type RejectedEvent struct {
	Reason      string
	ErrorNumber int
}

func (RejectedEvent) EventRule(reason string) *RejectedEvent {
	return &RejectedEvent{Reason: reason, ErrorNumber: 3001}
}

func (RejectedEvent) SendRequest(reason string, number int) *RejectedEvent {
	return &RejectedEvent{Reason: reason, ErrorNumber: number}
}

func (failure *RejectedEvent) Error() string {
	return failure.Reason
}

func (failure *RejectedEvent) Number() int {
	return failure.ErrorNumber
}
