package mailin

import (
	"errors"

	"mesail.dev/packages/outbox"
)

type OutboxStore struct {
	Storage  Storage
	Registry CollectionRegistry
}

func (store OutboxStore) Pending(limit int) ([]outbox.Document, error) {
	return store.matching(OutboxRow{}.Waiting(), limit)
}

func (store OutboxStore) Dead() ([]outbox.Document, error) {
	return store.matching(OutboxRow{}.HasDead(), store.Storage.Everything())
}

func (store OutboxStore) Load(ref outbox.Ref) (outbox.Fields, error) {
	err := store.Registry.Check(ref.Collection)
	if err != nil {
		return outbox.Fields{}, err
	}
	row := OutboxRow{}
	found, err := store.Storage.Find(ref.Collection, ref.Id, &row)
	if err != nil {
		return outbox.Fields{}, err
	}
	if !found {
		return outbox.Fields{}, Missing{Message: "no such outbox document"}
	}
	return row.Fields(), nil
}

func (store OutboxStore) Save(ref outbox.Ref, fields outbox.Fields) error {
	err := store.Registry.Check(ref.Collection)
	if err != nil {
		return err
	}
	return store.Storage.Replace(ref.Collection, ref.Id, OutboxPatch{}.From(fields))
}

func (store OutboxStore) matching(where map[string]any, limit int) ([]outbox.Document, error) {
	names, err := store.Registry.All()
	if err != nil {
		return []outbox.Document{}, err
	}
	found := []outbox.Document{}
	failures := []error{}
	for _, name := range names {
		if len(found) >= limit {
			break
		}
		documents, err := store.documentsIn(name, where, limit-len(found))
		found, failures = append(found, documents...), append(failures, err)
	}
	return found, errors.Join(failures...)
}

func (store OutboxStore) documentsIn(collection string, where map[string]any, limit int) ([]outbox.Document, error) {
	err := store.Storage.Ensure(collection)
	if err != nil {
		return nil, err
	}
	rows := []OutboxRow{}
	err = store.Storage.First(collection, where, limit, &rows)
	return store.documents(collection, rows), err
}

func (OutboxStore) documents(collection string, rows []OutboxRow) []outbox.Document {
	documents := []outbox.Document{}
	for _, row := range rows {
		documents = append(documents, outbox.Document{Ref: outbox.Ref{Collection: collection, Id: row.Id}, Fields: row.Fields()})
	}
	return documents
}
