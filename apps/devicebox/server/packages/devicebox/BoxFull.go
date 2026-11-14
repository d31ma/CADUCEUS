package devicebox

type BoxFull struct {
	Message     string
	ErrorNumber int
}

func (BoxFull) Device() *BoxFull {
	return &BoxFull{Message: "A receiving device already has 10,000 waiting envelopes.", ErrorNumber: 6013}
}

func (failure *BoxFull) Error() string {
	return failure.Message
}

func (failure *BoxFull) Number() int {
	return failure.ErrorNumber
}
