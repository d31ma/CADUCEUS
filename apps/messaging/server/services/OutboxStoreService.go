import (
	"cmp"
	"errors"

	"mesail.dev/packages/core"
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
	rows, failure := store.rowsOfAll()
	return store.first(store.kept(rows, store.pendingCount), limit), failure
}

func (store *OutboxStoreService) Load(ref outbox.Ref) (outbox.Fields, error) {
	failure := store.registry.Check(ref.Collection)
	row, found, readFailure := store.readUnless(failure, ref)
	return store.fields(row), cmp.Or(failure, readFailure, store.missing(found, failure, readFailure))
}

func (store *OutboxStoreService) Save(ref outbox.Ref, fields outbox.Fields) error {
	failure := store.registry.Check(ref.Collection)
	return cmp.Or(failure, store.saveUnless(failure, ref, fields))
}

func (store *OutboxStoreService) Dead() ([]outbox.Document, error) {
	rows, failure := store.rowsOfAll()
	return store.kept(rows, store.deadCount), failure
}

func (store *OutboxStoreService) rowsOfAll() ([]outbox.Document, error) {
	names, failure := store.registry.All()
	rows, readFailure := store.rowsOf(names)
	return rows, cmp.Or(failure, readFailure)
}

func (store *OutboxStoreService) rowsOf(names []string) ([]outbox.Document, error) {
	found := []outbox.Document{}
	failures := []error{}
	for _, name := range names {
		rows, failure := store.rowsIn(name)
		found, failures = append(found, rows...), append(failures, failure)
	}
	return found, errors.Join(failures...)
}

func (store *OutboxStoreService) rowsIn(collection string) ([]outbox.Document, error) {
	rows := []OutboxRow{}
	failure := store.documents.All(collection, map[string]any{}, &rows)
	return store.asDocuments(collection, rows), failure
}

func (store *OutboxStoreService) asDocuments(collection string, rows []OutboxRow) []outbox.Document {
	found := []outbox.Document{}
	for _, row := range rows {
		found = append(found, outbox.Document{Ref: outbox.Ref{Collection: collection, Id: row.Id}, Fields: store.fields(row)})
	}
	return found
}

func (store *OutboxStoreService) kept(rows []outbox.Document, count func(outbox.Fields) int) []outbox.Document {
	kept := []outbox.Document{}
	for _, row := range rows {
		kept = append(kept, store.ifCounted(row, count(row.Fields))...)
	}
	return kept
}

func (store *OutboxStoreService) ifCounted(row outbox.Document, count int) []outbox.Document {
	if count > 0 {
		return []outbox.Document{row}
	}
	return nil
}

func (store *OutboxStoreService) pendingCount(fields outbox.Fields) int {
	return fields.OutboxPending
}

func (store *OutboxStoreService) deadCount(fields outbox.Fields) int {
	return fields.OutboxDead
}

func (store *OutboxStoreService) first(rows []outbox.Document, limit int) []outbox.Document {
	return rows[:min(len(rows), max(limit, 0))]
}

func (store *OutboxStoreService) fields(row OutboxRow) outbox.Fields {
	return outbox.Fields{PendingEvents: row.PendingEvents, DeadEvents: row.DeadEvents, OutboxPending: row.OutboxPending, OutboxDead: row.OutboxDead}
}

func (store *OutboxStoreService) readUnless(failure error, ref outbox.Ref) (OutboxRow, bool, error) {
	row := OutboxRow{}
	if failure != nil {
		return row, false, nil
	}
	found, readFailure := store.documents.Find(ref.Collection, ref.Id, &row)
	return row, found, readFailure
}

func (store *OutboxStoreService) saveUnless(failure error, ref outbox.Ref, fields outbox.Fields) error {
	if failure != nil {
		return nil
	}
	return store.documents.Replace(ref.Collection, ref.Id, fields)
}

func (store *OutboxStoreService) missing(found bool, failures ...error) error {
	if found || errors.Join(failures...) != nil {
		return nil
	}
	return &core.NotFound{Message: "no such outbox document"}
}
