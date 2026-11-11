package attachments

type Conflict struct {
	Field  string
	Reason string
}

func (conflict Conflict) Error() string {
	return conflict.Field + ": " + conflict.Reason
}
