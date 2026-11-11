package mailout

type AttemptLog struct {
	Store DocumentStore
}

func (AttemptLog) base() string { return "delivery-attempts" }

func (log AttemptLog) Record(attempt DeliveryAttempt) error {
	collection, err := log.Store.Ready(log.base(), attempt.DomainId)
	if err != nil {
		return err
	}
	_, err = log.Store.Documents.Insert(collection, attempt)
	return err
}

func (log AttemptLog) ForItem(domainId string, itemId string) ([]DeliveryAttempt, error) {
	collection, err := log.Store.Ready(log.base(), domainId)
	if err != nil {
		return nil, err
	}
	documents, err := log.Store.All(collection, map[string]any{"itemId": itemId})
	if err != nil {
		return nil, err
	}
	return Decoder{}.Attempts(documents)
}
