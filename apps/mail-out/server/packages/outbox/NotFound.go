package outbox

type NotFound struct {
	Kind        string
	Key         string
	ErrorNumber int
}

func (NotFound) Document(kind string, key string) NotFound {
	return NotFound{Kind: kind, Key: key, ErrorNumber: 14}
}

func (NotFound) DeadLetter(key string) NotFound {
	return NotFound{Kind: "dead letter", Key: key, ErrorNumber: 15}
}

func (failure NotFound) Error() string {
	return "the " + failure.Kind + " " + failure.Key + " was not found"
}

func (failure NotFound) Number() int {
	return failure.ErrorNumber
}
