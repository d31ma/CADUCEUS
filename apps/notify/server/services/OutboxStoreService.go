import (
	"cmp"
	"encoding/json"

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
	return store.documentsWhere(store.where(outbox.Filters{}.Waiting()), limit)
}

func (store *OutboxStoreService) Dead(limit int) ([]outbox.Document, error) {
	return store.documentsWhere(store.where(outbox.Filters{}.HasDead()), limit)
}

func (store *OutboxStoreService) Load(ref outbox.Ref) (outbox.Fields, error) {
	failure := store.registry.Check(ref.Collection)
	row, found, failure := store.loadUnless(failure, ref)
	return store.fields(row), store.missingUnless(failure, found)
}

func (store *OutboxStoreService) Save(ref outbox.Ref, fields outbox.Fields) error {
	failure := store.registry.Check(ref.Collection)
	return store.saveUnless(failure, ref, store.patch(fields))
}

func (store *OutboxStoreService) documentsWhere(where map[string]any, limit int) ([]outbox.Document, error) {
	names, failure := store.registry.All(store.collectionLimit())
	found, problem := store.rowsOf(names, where, limit)
	return found[:min(len(found), limit)], cmp.Or(failure, problem)
}

func (store *OutboxStoreService) where(filter any) map[string]any {
	encoded, _ := json.Marshal(filter)
	where := map[string]any{}
	_ = json.Unmarshal(encoded, &where)
	return where
}

func (store *OutboxStoreService) rowsOf(names []string, where map[string]any, limit int) ([]outbox.Document, error) {
	found := []outbox.Document{}
	failures := []error{}
	for _, name := range names {
		rows := []OutboxRow{}
		failure := store.documents.Matching(name, where, limit, &rows)
		found, failures = append(found, store.asDocuments(name, rows)...), append(failures, failure)
	}
	return found, cmp.Or(failures...)
}

func (store *OutboxStoreService) asDocuments(collection string, rows []OutboxRow) []outbox.Document {
	found := []outbox.Document{}
	for _, row := range rows {
		found = append(found, outbox.Document{Ref: outbox.Ref{Collection: collection, Id: row.Id}, Fields: store.fields(row)})
	}
	return found
}

func (store *OutboxStoreService) loadUnless(failure error, ref outbox.Ref) (OutboxRow, bool, error) {
	if failure != nil {
		return OutboxRow{}, false, failure
	}
	row := OutboxRow{}
	found, failure := store.documents.Find(ref.Collection, ref.Id, &row)
	return row, found, failure
}

func (store *OutboxStoreService) missingUnless(failure error, found bool) error {
	if failure != nil || found {
		return failure
	}
	return &NotFound{What: "outbox document"}
}

func (store *OutboxStoreService) saveUnless(failure error, ref outbox.Ref, patch OutboxFieldsPatch) error {
	if failure != nil {
		return failure
	}
	return store.documents.Patch(ref.Collection, ref.Id, patch)
}

func (store *OutboxStoreService) patch(fields outbox.Fields) OutboxFieldsPatch {
	return OutboxFieldsPatch{
		PendingEvents: fields.PendingEvents,
		DeadEvents:    fields.DeadEvents,
		OutboxPending: fields.OutboxPending,
		OutboxDead:    fields.OutboxDead,
		OutboxWaiting: fields.OutboxPending > 0,
		OutboxHasDead: fields.OutboxDead > 0,
	}
}

func (store *OutboxStoreService) fields(row OutboxRow) outbox.Fields {
	return outbox.Fields{PendingEvents: row.PendingEvents, DeadEvents: row.DeadEvents, OutboxPending: row.OutboxPending, OutboxDead: row.OutboxDead, OutboxWaiting: row.OutboxWaiting, OutboxHasDead: row.OutboxHasDead}
}

func (store *OutboxStoreService) collectionLimit() int {
	return 1000
}
