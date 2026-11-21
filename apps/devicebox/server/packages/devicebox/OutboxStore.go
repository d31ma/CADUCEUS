package devicebox

import (
	"errors"

	"mesail.dev/packages/outbox"
)

type OutboxStore struct {
	documents Documents
	store     Store
}

func NewOutboxStore(documents Documents) OutboxStore {
	return OutboxStore{documents: documents, store: NewStore(documents)}
}

func (store OutboxStore) Waiting() ([]outbox.Document, error) {
	return store.everyFlagged("outboxWaiting")
}

func (store OutboxStore) Dead(limit int) ([]outbox.Document, error) {
	return store.flagged("outboxHasDead", limit)
}

func (store OutboxStore) Load(ref outbox.Ref) (outbox.Fields, error) {
	err := store.known(ref.Collection)
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
	err := store.known(ref.Collection)
	if err != nil {
		return err
	}
	return store.documents.Patch(ref.Collection, ref.Id, OutboxPatch{}.From(fields))
}

func (store OutboxStore) flagged(flag string, limit int) ([]outbox.Document, error) {
	records, err := store.store.Domains()
	if err != nil {
		return nil, err
	}
	found := []outbox.Document{}
	failures := []error{}
	for _, record := range records {
		rows, err := store.rowsIn(record.Name, flag, limit-len(found))
		found, failures = append(found, rows...), append(failures, err)
	}
	return store.first(found, limit), errors.Join(failures...)
}

func (store OutboxStore) everyFlagged(flag string) ([]outbox.Document, error) {
	records, err := store.store.Domains()
	if err != nil {
		return nil, err
	}
	found := []outbox.Document{}
	failures := []error{}
	for _, record := range records {
		rows, err := store.allIn(record.Name, flag)
		found, failures = append(found, rows...), append(failures, err)
	}
	return found, errors.Join(failures...)
}

func (store OutboxStore) allIn(collection string, flag string) ([]outbox.Document, error) {
	statement, err := SqlText{}.AllOutbox(collection, flag)
	if err != nil {
		return nil, err
	}
	rows := []OutboxRow{}
	err = store.documents.Rows(statement, &rows)
	return store.documentsOf(collection, rows), err
}

func (store OutboxStore) rowsIn(collection string, flag string, limit int) ([]outbox.Document, error) {
	if limit < 1 {
		return nil, nil
	}
	statement, err := SqlText{}.Outbox(collection, flag, limit)
	if err != nil {
		return nil, err
	}
	rows := []OutboxRow{}
	err = store.documents.Rows(statement, &rows)
	return store.documentsOf(collection, rows), err
}

func (store OutboxStore) documentsOf(collection string, rows []OutboxRow) []outbox.Document {
	found := []outbox.Document{}
	for _, row := range rows {
		found = append(found, outbox.Document{Ref: outbox.Ref{Collection: collection, Id: row.Id}, Fields: store.fields(row)})
	}
	return found
}

func (store OutboxStore) known(collection string) error {
	records, err := store.store.Domains()
	if err != nil {
		return err
	}
	return store.listed(records, collection)
}

func (OutboxStore) listed(records []RegistryRecord, collection string) error {
	for _, record := range records {
		if record.Name == collection {
			return nil
		}
	}
	return NotFound{}.OutboxCollection()
}

func (OutboxStore) fields(row OutboxRow) outbox.Fields {
	return outbox.Fields{PendingEvents: row.PendingEvents, DeadEvents: row.DeadEvents, OutboxPending: row.OutboxPending, OutboxDead: row.OutboxDead}
}

func (OutboxStore) missing(found bool) error {
	if found {
		return nil
	}
	return NotFound{}.OutboxDocument()
}

func (OutboxStore) first(rows []outbox.Document, limit int) []outbox.Document {
	if len(rows) > limit {
		return rows[:limit]
	}
	return rows
}
