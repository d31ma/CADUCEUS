package core

type EditConflict struct {
	Current     MessageView
	ErrorNumber int
}

func (EditConflict) Of(current MessageView) *EditConflict {
	return &EditConflict{Current: current, ErrorNumber: 1563}
}

func (failure *EditConflict) Error() string {
	return "the message changed since you read it"
}

func (failure *EditConflict) Number() int {
	return failure.ErrorNumber
}
