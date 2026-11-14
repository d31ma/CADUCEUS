package outbox

type attemptKind struct{}

func (attemptKind) skipped() string {
	return "skipped"
}

func (attemptKind) delivered() string {
	return "delivered"
}

func (attemptKind) dead() string {
	return "dead"
}

func (attemptKind) retry() string {
	return "retry"
}
