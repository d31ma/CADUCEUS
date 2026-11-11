package events

type UnknownEventType struct {
	Type string
}

func (failure UnknownEventType) Error() string {
	return "the event type " + failure.Type + " is not in the Mesail event catalog"
}
