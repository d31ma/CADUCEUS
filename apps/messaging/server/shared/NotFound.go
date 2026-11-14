type NotFound struct {
	Message     string
	ErrorNumber int
}

func (NotFound) OtherDomain() *NotFound {
	return &NotFound{Message: "no such domain", ErrorNumber: 1505}
}

func (failure *NotFound) Error() string {
	return failure.Message
}

func (failure *NotFound) Number() int {
	return failure.ErrorNumber
}
