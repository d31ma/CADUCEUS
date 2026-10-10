// TemplatesHelper reads and writes message templates.
// @Helper
type TemplatesHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores a template and answers its document id.
func (helper *TemplatesHelper) Put(entry map[string]any) (string, error) {
	return helper.templates().Put(entry)
}

// ListForOwner answers the owner's templates.
func (helper *TemplatesHelper) ListForOwner(owner string) ([]FyloRecord, error) {
	return helper.templates().Where("owner", owner)
}

// FindById answers the document id and template with a logical id, or ""
// and nil.
func (helper *TemplatesHelper) FindById(id string) (string, map[string]any, error) {
	return helper.templates().FindById(id)
}

// Update merges fields into one template.
func (helper *TemplatesHelper) Update(documentId string, patch map[string]any) error {
	return helper.templates().Patch(documentId, patch)
}

// Delete deletes one template.
func (helper *TemplatesHelper) Delete(documentId string) error {
	return helper.templates().Delete(documentId)
}

func (helper *TemplatesHelper) templates() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Templates}
}
