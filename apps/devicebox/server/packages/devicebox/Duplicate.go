package devicebox

type Duplicate struct {
	Message     string
	ErrorNumber int
}

func (Duplicate) EnvelopeId() *Duplicate {
	return &Duplicate{Message: "An envelope with this id already exists with other content.", ErrorNumber: 6012}
}

func (failure *Duplicate) Error() string {
	return failure.Message
}

func (failure *Duplicate) Number() int {
	return failure.ErrorNumber
}
