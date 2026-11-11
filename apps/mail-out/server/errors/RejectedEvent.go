type RejectedEvent struct {
	Reason string
}

func (failure *RejectedEvent) Error() string {
	return failure.Reason
}
