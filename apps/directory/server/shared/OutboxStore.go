import "mesail.dev/packages/outbox"

type OutboxStore struct {
	WaitingOf func() ([]outbox.Document, error)
	LoadOf    func(ref outbox.Ref) (outbox.Fields, error)
	SaveOf    func(ref outbox.Ref, fields outbox.Fields) error
}

func (store OutboxStore) Waiting() ([]outbox.Document, error) {
	return store.WaitingOf()
}

func (store OutboxStore) Load(ref outbox.Ref) (outbox.Fields, error) {
	return store.LoadOf(ref)
}

func (store OutboxStore) Save(ref outbox.Ref, fields outbox.Fields) error {
	return store.SaveOf(ref, fields)
}
