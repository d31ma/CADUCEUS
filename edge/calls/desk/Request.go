package desk

type Request struct {
	Token  string
	CallId string
	Decode func(into any) error
}
