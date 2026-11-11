import "mesail.dev/packages/outbox"

type OutboxStore struct {
	Service  *OutboxStoreService
	DomainId string
}

func (store OutboxStore) Pending(limit int) ([]outbox.Document, error) {
	return store.Service.PendingIn(store.DomainId, limit)
}

func (store OutboxStore) Load(ref outbox.Ref) (outbox.Fields, error) {
	return store.Service.Load(ref)
}

func (store OutboxStore) Save(ref outbox.Ref, fields outbox.Fields) error {
	return store.Service.Save(ref, fields)
}
