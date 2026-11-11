type Forbidden struct{}

func (failure *Forbidden) Error() string {
	return "this caller may not use this route"
}
