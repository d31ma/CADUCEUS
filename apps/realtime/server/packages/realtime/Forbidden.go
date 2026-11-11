package realtime

type Forbidden struct{}

func (Forbidden) Error() string {
	return "this caller may not use this route"
}
