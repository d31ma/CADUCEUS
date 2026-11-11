package attachments

type Unavailable struct {
	Operation string
}

func (unavailable Unavailable) Error() string {
	return "attachment storage failed during " + unavailable.Operation
}
