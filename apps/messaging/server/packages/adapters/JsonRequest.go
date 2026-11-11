package adapters

type JsonRequest struct {
	Method        string
	Url           string
	Authorization string
	Body          any
}
