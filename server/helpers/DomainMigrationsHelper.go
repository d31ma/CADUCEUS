// DomainMigrationsHelper keeps the record of each domain move, one per pair
// of old and new domain.
// @Helper
type DomainMigrationsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// List answers every migration as {docId, ...migration}, in FYLO's order.
func (helper *DomainMigrationsHelper) List() ([]map[string]any, error) {
	documents, err := helper.store().All()
	return StoredRecords{}.WithDocumentIds(documents), err
}

// Find answers the document id and migration from one domain to another, or
// "" and nil. FYLO joins the two conditions with OR, as the former query did,
// so the first document matching either one answers.
func (helper *DomainMigrationsHelper) Find(fromDomain string, toDomain string) (string, map[string]any, error) {
	store := helper.store()
	documents, err := store.Find(store.Condition(helper.fromField(), "$eq", fromDomain), store.Condition(helper.toField(), "$eq", toDomain))
	id, migration := StoredRecords{}.First(documents)
	return id, migration, err
}

// Put stores a migration, merging it into the stored one for the same pair,
// and answers its document id.
func (helper *DomainMigrationsHelper) Put(migration map[string]any) (string, error) {
	documentId, _, err := helper.Find(RecordValues{}.Text(migration[helper.fromField()]), RecordValues{}.Text(migration[helper.toField()]))
	if err != nil {
		return "", err
	}
	if documentId != "" {
		return documentId, helper.store().Patch(documentId, migration)
	}
	return helper.store().Put(migration)
}

func (DomainMigrationsHelper) fromField() string { return "fromDomain" }

func (DomainMigrationsHelper) toField() string { return "toDomain" }

func (helper *DomainMigrationsHelper) store() CollectionStore {
	return CollectionStore{Database: helper.fyloHelper.Database(), Collection: FyloCollections{}.Named().DomainMigrations}
}
