package search

type Unavailable struct {
	Message string
	Cause   error
}

func (failure Unavailable) Error() string {
	return failure.Message + failure.causeText()
}

func (failure Unavailable) Unwrap() error {
	return failure.Cause
}

func (failure Unavailable) causeText() string {
	if failure.Cause == nil {
		return ""
	}
	return ": " + failure.Cause.Error()
}
