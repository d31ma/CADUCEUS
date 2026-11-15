type Forbidden struct {
	Message     string
	ErrorNumber int
}

func (Forbidden) NotADomainMember() *Forbidden {
	return &Forbidden{Message: "only members of a domain use its conversations", ErrorNumber: 1502}
}

func (Forbidden) NotAService() *Forbidden {
	return &Forbidden{Message: "only a caller with the Service role may use this route", ErrorNumber: 1503}
}

func (Forbidden) NotInstallWide() *Forbidden {
	return &Forbidden{Message: "the outbox is managed with an install-wide token only", ErrorNumber: 1504}
}

func (Forbidden) Kit(message string, number int) *Forbidden {
	return &Forbidden{Message: message, ErrorNumber: number}
}

func (failure *Forbidden) Error() string {
	return failure.Message
}

func (failure *Forbidden) Number() int {
	return failure.ErrorNumber
}
