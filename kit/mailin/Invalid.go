package mailin

type Invalid struct {
	Field  string
	Reason string
}

func (invalid Invalid) Error() string {
	return invalid.Field + ": " + invalid.Reason
}
