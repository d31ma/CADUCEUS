package mailin

type Unavailable struct {
	Operation string
}

func (unavailable Unavailable) Error() string {
	return "inbound storage failed: " + unavailable.Operation
}
