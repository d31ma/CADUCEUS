package addresses

type InvalidName struct {
	Kind   string
	Reason string
}

func (invalid InvalidName) Error() string {
	return invalid.Kind + " " + invalid.Reason
}
