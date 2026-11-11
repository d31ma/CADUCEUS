package channels

import (
	"errors"

	"mesail.dev/packages/outbox"
)

type OutboxStore struct {
	documents Documents
	registry  Registry
}

func NewOutboxStore(documents Documents) OutboxStore {
	return OutboxStore{documents: documents, registry: NewRegistry(documents)}
}

func (store OutboxStore) Pending(limit int) ([]outbox.Document, error) {
	rows, err := store.rows(map[string]any{store.waitingField(): true})
	return store.first(store.kept(rows, store.pendingCount), limit), err
}

func (store OutboxStore) Dead() ([]outbox.Document, error) {
	rows, err := store.rows(map[string]any{store.deadField(): true})
	return store.kept(rows, store.deadCount), err
}

func (store OutboxStore) Load(ref outbox.Ref) (outbox.Fields, error) {
	err := store.registry.Check(ref.Collection)
	if err != nil {
		return outbox.Fields{}, err
	}
	row := OutboxRow{}
	found, err := store.documents.Find(ref.Collection, ref.Id, &row)
	if err != nil {
		return outbox.Fields{}, err
	}
	return store.fields(row), store.missing(found)
}

func (store OutboxStore) Save(ref outbox.Ref, fields outbox.Fields) error {
	err := store.registry.Check(ref.Collection)
	if err != nil {
		return err
	}
	return store.documents.Replace(ref.Collection, ref.Id, OutboxPatch{}.From(fields))
}

func (store OutboxStore) rows(where map[string]any) ([]outbox.Document, error) {
	names, err := store.registry.All()
	if err != nil {
		return nil, err
	}
	found := []outbox.Document{}
	failures := []error{}
	for _, name := range names {
		rows, err := store.rowsIn(name, where)
		found, failures = append(found, rows...), append(failures, err)
	}
	return found, errors.Join(failures...)
}

func (store OutboxStore) rowsIn(collection string, where map[string]any) ([]outbox.Document, error) {
	rows := []OutboxRow{}
	err := store.documents.All(collection, where, &rows)
	found := []outbox.Document{}
	for _, row := range rows {
		found = append(found, outbox.Document{Ref: outbox.Ref{Collection: collection, Id: row.Id}, Fields: store.fields(row)})
	}
	return found, err
}

func (store OutboxStore) kept(rows []outbox.Document, count func(outbox.Fields) int) []outbox.Document {
	kept := []outbox.Document{}
	for _, row := range rows {
		kept = append(kept, store.ifCounted(row, count(row.Fields))...)
	}
	return kept
}

func (OutboxStore) ifCounted(row outbox.Document, count int) []outbox.Document {
	if count > 0 {
		return []outbox.Document{row}
	}
	return nil
}

func (OutboxStore) pendingCount(fields outbox.Fields) int {
	return fields.OutboxPending
}

func (OutboxStore) deadCount(fields outbox.Fields) int {
	return fields.OutboxDead
}

func (OutboxStore) waitingField() string {
	return "outboxWaiting"
}

func (OutboxStore) deadField() string {
	return "outboxHasDead"
}

func (OutboxStore) first(rows []outbox.Document, limit int) []outbox.Document {
	if len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func (OutboxStore) fields(row OutboxRow) outbox.Fields {
	return outbox.Fields{PendingEvents: row.PendingEvents, DeadEvents: row.DeadEvents, OutboxPending: row.OutboxPending, OutboxDead: row.OutboxDead}
}

func (OutboxStore) missing(found bool) error {
	if found {
		return nil
	}
	return &NotFound{Message: "no such outbox document"}
}
