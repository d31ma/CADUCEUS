package inbound

type UnhandledEvent struct {
	Type string
}

func (failure UnhandledEvent) Error() string {
	return "messaging does not handle the event type " + failure.Type
}
