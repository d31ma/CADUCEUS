// CollectionStore runs the FYLO operations of one collection the way the
// JavaScript repositories sent them: queries as {"$ops": [...]}, where FYLO
// joins several conditions with OR, and patches as one patchDoc per document.
type CollectionStore struct {
	Database   FyloDatabase
	Collection string
}

// All answers every document of the collection, in FYLO's order.
func (store CollectionStore) All() ([]FyloDocument, error) {
	return store.Find()
}

// Equal answers the documents whose field equals value.
func (store CollectionStore) Equal(field string, value any) ([]FyloDocument, error) {
	return store.Find(store.Condition(field, "$eq", value))
}

// EqualRecords answers the documents whose field equals value as
// {docId, ...record}, in FYLO's order.
func (store CollectionStore) EqualRecords(field string, value any) ([]map[string]any, error) {
	documents, err := store.Equal(field, value)
	return StoredRecords{}.WithDocumentIds(documents), err
}

// Find answers the documents that match any of the conditions.
func (store CollectionStore) Find(conditions ...map[string]any) ([]FyloDocument, error) {
	operations := []any{}
	for _, condition := range conditions {
		operations = append(operations, condition)
	}
	return store.Database.FindDocs(store.Collection, map[string]any{"$ops": operations})
}

// Condition answers one FYLO condition, such as {"id": {"$eq": "a"}}.
func (CollectionStore) Condition(field string, operator string, value any) map[string]any {
	return map[string]any{field: map[string]any{operator: value}}
}

// Put stores a new document and answers its id.
func (store CollectionStore) Put(data any) (string, error) {
	return store.Database.PutData(store.Collection, data)
}

// Patch merges fields into one document. The FYLO machine reads the
// document id as text and its new fields from newDoc, which is what the
// JavaScript client sent for each document of a {docId: fields} patch.
func (store CollectionStore) Patch(id string, fields any) error {
	_, err := store.Database.Op("patchDoc", map[string]any{"collection": store.Collection, "id": id, "newDoc": fields})
	return err
}

// Delete deletes one document.
func (store CollectionStore) Delete(id string) error {
	return store.Database.DelDoc(store.Collection, id)
}

// ForDomain answers the same collection acting as the service member of one
// managed domain's tenant storage (the former FyloAccessContext.runForDomain).
// With POSIX access on, a domain that is not valid is refused.
func (store CollectionStore) ForDomain(domain string) (CollectionStore, error) {
	normalized := Security{}.NormalizeDomain(domain)
	access := FyloAccess{}.ForDomain(normalized)
	if access != nil && normalized == "" {
		return store, &BoundaryError{Code: "EACCES", Message: "A valid domain is required for tenant storage"}
	}
	return store.As(access), nil
}

// As answers the same collection acting with another access descriptor.
func (store CollectionStore) As(access *FyloAccess) CollectionStore {
	store.Database = store.Database.As(access)
	return store
}
