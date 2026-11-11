package realtime

type Invalid struct {
	Field  string
	Reason string
}

func (failure Invalid) Error() string {
	return "invalid " + failure.Field + ": " + failure.Reason
}
