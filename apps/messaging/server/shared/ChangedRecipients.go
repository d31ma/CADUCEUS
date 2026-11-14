type ChangedRecipients struct {
	Message     string
	Addresses   []string
	ErrorNumber int
}

func (ChangedRecipients) Of(addresses []string, number int) *ChangedRecipients {
	return &ChangedRecipients{Message: "the recipients changed", Addresses: addresses, ErrorNumber: number}
}

func (failure *ChangedRecipients) Error() string {
	return failure.Message
}

func (failure *ChangedRecipients) Number() int {
	return failure.ErrorNumber
}
