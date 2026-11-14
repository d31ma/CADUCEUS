package events

type NotSubscribed struct {
	Service string
	Type    string
}

func (failure NotSubscribed) Error() string {
	return "the service " + failure.Service + " does not consume events of type " + failure.Type
}
