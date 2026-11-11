package mailout

import (
	"mesail.dev/packages/outbox"
)

type OutboxStore struct {
	Store    DocumentStore
	Registry Registry
}

func (OutboxStore) largestPage() int { return 100 }

func (store OutboxStore) Pending(limit int) ([]outbox.Document, error) {
	return store.flagged(map[string]any{"outboxWaiting": true}, limit)
}

func (store OutboxStore) WithDeadLetters(limit int) ([]outbox.Document, error) {
	return store.flagged(map[string]any{"outboxHasDead": true}, limit)
}

func (store OutboxStore) Load(ref outbox.Ref) (outbox.Fields, error) {
	item, err := store.item(ref)
	return item.OutboxFields(), err
}

func (store OutboxStore) Owner(ref outbox.Ref) (string, error) {
	item, err := store.item(ref)
	return item.DomainId, err
}

func (store OutboxStore) Save(ref outbox.Ref, fields outbox.Fields) error {
	_, err := store.Store.Documents.Patch(ref.Collection, ref.Id, NewOutboxPatch(fields), &Item{})
	return err
}

func (store OutboxStore) item(ref outbox.Ref) (Item, error) {
	item := Item{}
	found, err := store.Store.Documents.FindById(ref.Collection, ref.Id, &item)
	if err == nil && !found {
		return Item{}, outbox.NotFound{Kind: "outbound item", Key: ref.Collection + "/" + ref.Id}
	}
	return item, err
}

func (store OutboxStore) flagged(where map[string]any, limit int) ([]outbox.Document, error) {
	entries, err := store.Registry.Collections(Queue{}.Base())
	if err != nil {
		return nil, err
	}
	documents := []outbox.Document{}
	for _, entry := range entries {
		page, _, err := store.Store.Page(entry.Collection, where, "", min(limit, store.largestPage()))
		if err != nil {
			return nil, err
		}
		items, err := Decoder{}.Items(page)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			documents = append(documents, outbox.Document{Ref: outbox.Ref{Collection: entry.Collection, Id: item.Id}, Fields: item.OutboxFields()})
		}
	}
	return documents[:min(len(documents), limit)], nil
}
