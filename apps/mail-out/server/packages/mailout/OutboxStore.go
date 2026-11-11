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
	return store.flagged(outbox.Filters{}.Waiting(), limit)
}

func (store OutboxStore) WithDeadLetters(limit int) ([]outbox.Document, error) {
	return store.flagged(outbox.Filters{}.HasDead(), limit)
}

func (store OutboxStore) Load(ref outbox.Ref) (outbox.Fields, error) {
	item, err := store.item(ref)
	return item.OutboxFields(), err
}

func (store OutboxStore) Save(ref outbox.Ref, fields outbox.Fields) error {
	_, err := store.Store.Documents.Patch(ref.Collection, ref.Id, NewOutboxPatch(fields), &Item{})
	return err
}

func (store OutboxStore) item(ref outbox.Ref) (Item, error) {
	item := Item{}
	found, err := store.Store.Documents.FindById(ref.Collection, ref.Id, &item)
	if err != nil {
		return item, err
	}
	return store.foundItem(item, found, ref)
}

func (OutboxStore) foundItem(item Item, found bool, ref outbox.Ref) (Item, error) {
	if !found {
		return Item{}, outbox.NotFound{Kind: "outbound item", Key: ref.Collection + "/" + ref.Id}
	}
	return item, nil
}

func (store OutboxStore) flagged(filter any, limit int) ([]outbox.Document, error) {
	entries, err := store.Registry.Collections(Queue{}.Base())
	if err != nil {
		return nil, err
	}
	documents, err := store.collect(entries, filter, limit)
	if err != nil {
		return nil, err
	}
	return documents[:min(len(documents), limit)], nil
}

func (store OutboxStore) collect(entries []CollectionEntry, filter any, limit int) ([]outbox.Document, error) {
	documents := []outbox.Document{}
	for _, entry := range entries {
		found, err := store.inCollection(entry.Collection, filter, limit)
		if err != nil {
			return nil, err
		}
		documents = append(documents, found...)
	}
	return documents, nil
}

func (store OutboxStore) inCollection(collection string, filter any, limit int) ([]outbox.Document, error) {
	page, _, err := store.Store.Page(collection, filter, "", min(limit, store.largestPage()))
	if err != nil {
		return nil, err
	}
	items, err := Decoder{}.Items(page)
	if err != nil {
		return nil, err
	}
	return store.documents(collection, items), nil
}

func (OutboxStore) documents(collection string, items []Item) []outbox.Document {
	documents := []outbox.Document{}
	for _, item := range items {
		documents = append(documents, outbox.Document{Ref: outbox.Ref{Collection: collection, Id: item.Id}, Fields: item.OutboxFields()})
	}
	return documents
}
