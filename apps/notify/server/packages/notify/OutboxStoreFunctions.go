package notify

import "mesail.dev/packages/outbox"

type OutboxStoreFunctions struct {
	WaitingFunction func() ([]outbox.Document, error)
	LoadFunction    func(ref outbox.Ref) (outbox.Fields, error)
	SaveFunction    func(ref outbox.Ref, fields outbox.Fields) error
}

func (store OutboxStoreFunctions) Waiting() ([]outbox.Document, error) {
	return store.WaitingFunction()
}

func (store OutboxStoreFunctions) Load(ref outbox.Ref) (outbox.Fields, error) {
	return store.LoadFunction(ref)
}

func (store OutboxStoreFunctions) Save(ref outbox.Ref, fields outbox.Fields) error {
	return store.SaveFunction(ref, fields)
}
