package realtime

type Missing struct {
	What        string
	ErrorNumber int
}

func (Missing) EventDomain() Missing {
	return Missing{What: "domain", ErrorNumber: 5019}
}

func (Missing) SweepDomain() Missing {
	return Missing{What: "domain", ErrorNumber: 5020}
}

func (Missing) Presence() Missing {
	return Missing{What: "presence", ErrorNumber: 5021}
}

func (Missing) Conversation() Missing {
	return Missing{What: "conversation", ErrorNumber: 5022}
}

func (Missing) Connection() Missing {
	return Missing{What: "connection", ErrorNumber: 5023}
}

func (failure Missing) Error() string {
	return "not found: " + failure.What
}

func (failure Missing) Number() int {
	return failure.ErrorNumber
}
