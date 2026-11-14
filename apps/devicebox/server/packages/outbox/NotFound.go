package outbox

type NotFound struct {
	Kind string
	Key  string
}

func (failure NotFound) Error() string {
	return "the " + failure.Kind + " " + failure.Key + " was not found"
}
