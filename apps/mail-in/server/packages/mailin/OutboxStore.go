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
	documents, err := store.matching(OutboxRow.IsWaiting)
	return documents[:min(limit, len(documents))], err
}

func (store OutboxStore) Dead() ([]outbox.Document, error) {
	return store.matching(OutboxRow.IsDead)
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
	return store.Storage.Replace(ref.Collection, ref.Id, fields)
}

func (store OutboxStore) matching(keep func(OutboxRow) bool) ([]outbox.Document, error) {
	names, err := store.Registry.All()
	if err != nil {
		return []outbox.Document{}, err
	}
	found := []outbox.Document{}
	failures := []error{}
	for _, name := range names {
		documents, err := store.documentsIn(name, keep)
		found, failures = append(found, documents...), append(failures, err)
	}
	return found, errors.Join(failures...)
}

func (store OutboxStore) documentsIn(collection string, keep func(OutboxRow) bool) ([]outbox.Document, error) {
	err := store.Storage.Ensure(collection)
	if err != nil {
		return nil, err
	}
	rows := []OutboxRow{}
	err = store.Storage.All(collection, map[string]any{}, &rows)
	return store.kept(collection, rows, keep), err
}

func (OutboxStore) kept(collection string, rows []OutboxRow, keep func(OutboxRow) bool) []outbox.Document {
	documents := []outbox.Document{}
	for _, row := range rows {
		if keep(row) {
			documents = append(documents, outbox.Document{Ref: outbox.Ref{Collection: collection, Id: row.Id}, Fields: row.Fields()})
		}
	}
	return documents
}
