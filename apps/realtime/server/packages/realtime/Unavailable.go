package realtime

type Unavailable struct {
	Operation string
}

func (failure Unavailable) Error() string {
	return "realtime storage or gateway failed: " + failure.Operation
}
