package mailout

type AttemptLog struct {
	Store DocumentStore
}

func (AttemptLog) base() string { return "delivery-attempts" }

func (AttemptLog) itemField() string { return "itemId" }

func (log AttemptLog) Record(attempt DeliveryAttempt) error {
	collection := log.Store.Name(log.base(), attempt.DomainId)
	_, err := log.Store.Insert(collection, attempt)
	return err
}

func (log AttemptLog) ForItem(domainId string, itemId string) ([]DeliveryAttempt, error) {
	collection := log.Store.Name(log.base(), domainId)
	documents, err := log.Store.All(collection, map[string]any{log.itemField(): itemId})
	if err != nil {
		return nil, err
	}
	return Decoder{}.Attempts(documents)
}
