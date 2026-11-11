import (
	"errors"

	"mesail.dev/packages/outbox"
)

// @Service
type OutboxStoreService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	registry *OutboxCollectionsService
}

func (store *OutboxStoreService) Pending(limit int) ([]outbox.Document, error) {
	rows, err := store.rowsOfAll()
	return store.first(store.waiting(rows), limit), err
}

func (store *OutboxStoreService) Load(ref outbox.Ref) (outbox.Fields, error) {
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

func (store *OutboxStoreService) Save(ref outbox.Ref, fields outbox.Fields) error {
	err := store.registry.Check(ref.Collection)
	if err != nil {
		return err
	}
	return store.documents.Replace(ref.Collection, ref.Id, fields)
}

func (store *OutboxStoreService) Dead() ([]outbox.Document, error) {
	rows, err := store.rowsOfAll()
	return store.dead(rows), err
}

func (store *OutboxStoreService) rowsOfAll() ([]outbox.Document, error) {
	names, err := store.registry.All()
	if err != nil {
		return nil, err
	}
	return store.rowsOf(names)
}

func (store *OutboxStoreService) rowsOf(names []string) ([]outbox.Document, error) {
	found := []outbox.Document{}
	failures := []error{}
	for _, name := range names {
		rows, err := store.rowsIn(name)
		found, failures = append(found, rows...), append(failures, err)
	}
	return found, errors.Join(failures...)
}

func (store *OutboxStoreService) rowsIn(collection string) ([]outbox.Document, error) {
	err := store.documents.Ensure(collection)
	if err != nil {
		return nil, err
	}
	rows := []OutboxRow{}
	err = store.documents.All(collection, map[string]any{}, &rows)
	return store.asDocuments(collection, rows), err
}

func (store *OutboxStoreService) asDocuments(collection string, rows []OutboxRow) []outbox.Document {
	found := []outbox.Document{}
	for _, row := range rows {
		found = append(found, outbox.Document{Ref: outbox.Ref{Collection: collection, Id: row.Id}, Fields: store.fields(row)})
	}
	return found
}

func (store *OutboxStoreService) waiting(rows []outbox.Document) []outbox.Document {
	kept := []outbox.Document{}
	for _, row := range rows {
		kept = append(kept, store.ifWaiting(row)...)
	}
	return kept
}

func (store *OutboxStoreService) ifWaiting(row outbox.Document) []outbox.Document {
	if row.Fields.OutboxPending > 0 {
		return []outbox.Document{row}
	}
	return nil
}

func (store *OutboxStoreService) dead(rows []outbox.Document) []outbox.Document {
	kept := []outbox.Document{}
	for _, row := range rows {
		kept = append(kept, store.ifDead(row)...)
	}
	return kept
}

func (store *OutboxStoreService) ifDead(row outbox.Document) []outbox.Document {
	if row.Fields.OutboxDead > 0 {
		return []outbox.Document{row}
	}
	return nil
}

func (store *OutboxStoreService) first(rows []outbox.Document, limit int) []outbox.Document {
	if len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func (store *OutboxStoreService) fields(row OutboxRow) outbox.Fields {
	return outbox.Fields{PendingEvents: row.PendingEvents, DeadEvents: row.DeadEvents, OutboxPending: row.OutboxPending, OutboxDead: row.OutboxDead}
}

func (store *OutboxStoreService) missing(found bool) error {
	if found {
		return nil
	}
	return &NotFound{Message: "no such outbox document"}
}
