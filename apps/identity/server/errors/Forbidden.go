type Forbidden struct {
	Code string
}

func (failure *Forbidden) Error() string {
	return "this session may not open that domain"
}
