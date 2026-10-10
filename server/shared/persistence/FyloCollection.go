// FyloCollection runs the document operations the repositories share on one
// collection, answering decoded records in FYLO's order.
type FyloCollection struct {
	Database FyloDatabase
	Name     string
}

// Put stores one document and answers its id.
func (collection FyloCollection) Put(data any) (string, error) {
	return collection.Database.PutData(collection.Name, data)
}

// All answers every document of the collection.
func (collection FyloCollection) All() ([]FyloRecord, error) {
	return collection.records(collection.Database.FindDocs(collection.Name, map[string]any{"$ops": []any{}}))
}

// Where answers the documents whose field equals value.
func (collection FyloCollection) Where(field string, value any) ([]FyloRecord, error) {
	return collection.records(collection.Database.FindDocs(collection.Name, collection.Equal(field, value)))
}

// First answers the first document whose field equals value, or an empty
// record (Id "") when there is none.
func (collection FyloCollection) First(field string, value any) (FyloRecord, error) {
	records, err := collection.Where(field, value)
	if err != nil || len(records) == 0 {
		return FyloRecord{}, err
	}
	return records[0], nil
}

// Equal answers the FYLO query for documents whose field equals value.
func (FyloCollection) Equal(field string, value any) map[string]any {
	return map[string]any{"$ops": []any{map[string]any{field: map[string]any{"$eq": value}}}}
}

// Patch merges fields into one document.
func (collection FyloCollection) Patch(id string, fields any) error {
	return collection.Database.PatchDoc(collection.Name, id, fields)
}

// PatchEach merges each record's fields into its document, one operation
// after another in order, as the JavaScript client patched a
// `{ docId: fields }` object.
func (collection FyloCollection) PatchEach(patches []FyloRecord) error {
	for _, patch := range patches {
		if err := collection.Patch(patch.Id, patch.Data); err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes one document.
func (collection FyloCollection) Delete(id string) error {
	return collection.Database.DelDoc(collection.Name, id)
}

func (FyloCollection) records(documents []FyloDocument, err error) ([]FyloRecord, error) {
	if err != nil {
		return nil, err
	}
	records := make([]FyloRecord, 0, len(documents))
	for _, document := range documents {
		records = append(records, FyloRecord{}.Decoded(document))
	}
	return records, nil
}
