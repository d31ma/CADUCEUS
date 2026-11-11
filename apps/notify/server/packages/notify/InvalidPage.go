package notify

type InvalidPage struct {
	Reason string
}

func (failure InvalidPage) Error() string {
	return "the page request is not valid: " + failure.Reason
}
