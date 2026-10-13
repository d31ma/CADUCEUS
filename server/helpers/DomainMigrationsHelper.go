// DomainMigrationsHelper reads the record of each domain move, one per pair
// of old and new domain (scripts/migrate-domain.mjs writes them).
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

func (helper *DomainMigrationsHelper) store() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().DomainMigrations}
}
