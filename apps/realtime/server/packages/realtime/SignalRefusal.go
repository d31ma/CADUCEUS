package realtime

type SignalRefusal struct {
	Code        string
	Field       string
	Text        string
	ErrorNumber int
}

func (failure SignalRefusal) Error() string {
	return failure.Code + " " + failure.Field + ": " + failure.Text
}

func (failure SignalRefusal) Number() int {
	return failure.ErrorNumber
}
