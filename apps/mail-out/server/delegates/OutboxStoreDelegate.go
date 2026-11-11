import (
	"errors"

	"mesail.dev/packages/outbox"
)

// @Delegate
// @NeedsAll()
type OutboxStoreDelegate struct {
	dataSource DataSource
	registry   *CollectionRegistryDelegate
}

func (OutboxStoreDelegate) largestPage() int { return 100 }

func (OutboxStoreDelegate) kind() string { return "outbound item" }

func (store *OutboxStoreDelegate) Pending(limit int) ([]outbox.Document, error) {
	return store.flagged(map[string]any{"outboxWaiting": true}, limit)
}

func (store *OutboxStoreDelegate) WithDeadLetters(limit int) ([]outbox.Document, error) {
	return store.flagged(map[string]any{"outboxHasDead": true}, limit)
}

func (store *OutboxStoreDelegate) Load(ref outbox.Ref) (outbox.Fields, error) {
	item := OutboundItem{}
	found, err := store.dataSource.FindById(ref.Collection, ref.Id, &item)
	return OutboxUtils{}.Fields(item), store.missing(found, ref, err)
}

func (store *OutboxStoreDelegate) Owner(ref outbox.Ref) (string, error) {
	item := OutboundItem{}
	found, err := store.dataSource.FindById(ref.Collection, ref.Id, &item)
	return item.DomainId, store.missing(found, ref, err)
}

func (store *OutboxStoreDelegate) Save(ref outbox.Ref, fields outbox.Fields) error {
	_, err := store.dataSource.Patch(ref.Collection, ref.Id, OutboxUtils{}.Patch(fields), &OutboundItem{})
	return err
}

func (store *OutboxStoreDelegate) flagged(where map[string]any, limit int) ([]outbox.Document, error) {
	entries, err := store.registry.Collections(StorageUtils{}.OutboundQueue())
	documents := []outbox.Document{}
	for _, entry := range entries {
		window, scrollErr := store.dataSource.Scroll(entry.Collection, where, "", min(limit, store.largestPage()))
		items := []OutboundItem{}
		decodeErr := window.Decode(&items)
		documents, err = append(documents, OutboxUtils{}.Documents(entry.Collection, items)...), errors.Join(err, scrollErr, decodeErr)
	}
	return documents[:min(len(documents), limit)], err
}

func (store *OutboxStoreDelegate) missing(found bool, ref outbox.Ref, failure error) error {
	if !found && failure == nil {
		return outbox.NotFound{Kind: store.kind(), Key: ref.Collection + "/" + ref.Id}
	}
	return failure
}
