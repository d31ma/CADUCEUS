import (
	"bytes"
	"encoding/json"
)

// StoredRecords reads FYLO documents as the plain records the JavaScript
// repositories passed around: decoded JSON objects, optionally carrying the
// document id under "docId".
type StoredRecords struct{}

// Decoded answers one document as a record (empty when it is not an object).
func (StoredRecords) Decoded(raw json.RawMessage) map[string]any {
	record := map[string]any{}
	_ = json.Unmarshal(raw, &record)
	if record == nil {
		record = map[string]any{}
	}
	return record
}

// Listed answers every document as a record, in FYLO's order.
func (records StoredRecords) Listed(documents []FyloDocument) []map[string]any {
	listed := []map[string]any{}
	for _, document := range documents {
		listed = append(listed, records.Decoded(document.Data))
	}
	return listed
}

// WithDocumentIds answers every document as {docId, ...record}: a docId the
// record itself stores wins, as it did in the JavaScript spread.
func (records StoredRecords) WithDocumentIds(documents []FyloDocument) []map[string]any {
	listed := []map[string]any{}
	for _, document := range documents {
		listed = append(listed, records.WithDocumentId(document.Id, records.Decoded(document.Data)))
	}
	return listed
}

// WithDocumentId answers {docId, ...record}.
func (StoredRecords) WithDocumentId(id string, record map[string]any) map[string]any {
	copied := map[string]any{"docId": id}
	for key, value := range record {
		copied[key] = value
	}
	return copied
}

// First answers the id and record of the first document, or "" and nil.
func (records StoredRecords) First(documents []FyloDocument) (string, map[string]any) {
	if len(documents) == 0 {
		return "", nil
	}
	return documents[0].Id, records.Decoded(documents[0].Data)
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
