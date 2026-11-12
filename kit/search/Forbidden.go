package search

type Forbidden struct {
	Message     string
	ErrorNumber int
}

func (Forbidden) MembersOnly() Forbidden {
	return Forbidden{Message: "only members of a domain search its messages", ErrorNumber: 4013}
}

func (Forbidden) ServicesOnly(message string) Forbidden {
	return Forbidden{Message: message, ErrorNumber: 4014}
}

func (failure Forbidden) Error() string {
	return failure.Message
}
