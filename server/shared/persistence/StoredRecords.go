import (
	"bytes"
	"encoding/json"
)

// StoredRecords reads FYLO documents as the plain records the JavaScript
// repositories passed around: decoded JSON objects, optionally carrying the
// document id under "docId".
type StoredRecords struct{}

// Listed answers the fields of every record, in FYLO's order.
func (StoredRecords) Listed(records []FyloRecord) []map[string]any {
	listed := []map[string]any{}
	for _, record := range records {
		listed = append(listed, record.Data)
	}
	return listed
}

// WithDocumentIds answers every record as {docId, ...record}: a docId the
// record itself stores wins, as it did in the JavaScript spread.
func (StoredRecords) WithDocumentIds(records []FyloRecord) []map[string]any {
	listed := []map[string]any{}
	for _, record := range records {
		listed = append(listed, record.WithDocId())
	}
	return listed
}

// Where answers the records of the collection whose field equals value, as
// {docId, ...record}.
func (records StoredRecords) Where(collection FyloCollection, field string, value any) ([]map[string]any, error) {
	found, err := collection.Where(field, value)
	return records.WithDocumentIds(found), err
}

// Without answers a copy of record without the named fields.
func (StoredRecords) Without(record map[string]any, fields ...string) map[string]any {
	copied := map[string]any{}
	for key, value := range record {
		copied[key] = value
	}
	for _, field := range fields {
		delete(copied, field)
	}
	return copied
}

// JsonText answers value as JSON.stringify wrote it: compact, without HTML
// escaping.
func (StoredRecords) JsonText(value any) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(value) != nil {
		return ""
	}
	return string(bytes.TrimRight(buffer.Bytes(), "\n"))
}

// Defined answers fields without the ones that are undefined, as
// JSON.stringify left them out of a payload or a stored record.
func (StoredRecords) Defined(fields map[string]any) map[string]any {
	for key, value := range fields {
		if value == nil {
			delete(fields, key)
		}
	}
	return fields
}
