package notify

type SeenStoreFunctions struct {
	SeenFunction     func(domainId string, eventId string) (bool, error)
	RememberFunction func(domainId string, eventId string) error
}

func (store SeenStoreFunctions) Seen(domainId string, eventId string) (bool, error) {
	return store.SeenFunction(domainId, eventId)
}

func (store SeenStoreFunctions) Remember(domainId string, eventId string) error {
	return store.RememberFunction(domainId, eventId)
}
