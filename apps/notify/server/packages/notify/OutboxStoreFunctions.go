package notify

import "mesail.dev/packages/outbox"

type OutboxStoreFunctions struct {
	PendingFunction func(limit int) ([]outbox.Document, error)
	LoadFunction    func(ref outbox.Ref) (outbox.Fields, error)
	SaveFunction    func(ref outbox.Ref, fields outbox.Fields) error
}

func (store OutboxStoreFunctions) Pending(limit int) ([]outbox.Document, error) {
	return store.PendingFunction(limit)
}

func (store OutboxStoreFunctions) Load(ref outbox.Ref) (outbox.Fields, error) {
	return store.LoadFunction(ref)
}

func (store OutboxStoreFunctions) Save(ref outbox.Ref, fields outbox.Fields) error {
	return store.SaveFunction(ref, fields)
}
