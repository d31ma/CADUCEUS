import "encoding/json"

// FyloRecord is one decoded FYLO document and its document id.
type FyloRecord struct {
	Id   string
	Data map[string]any
}

// Decoded answers a stored document as fields (empty when it is not an
// object).
func (FyloRecord) Decoded(document FyloDocument) FyloRecord {
	data := map[string]any{}
	_ = json.Unmarshal(document.Data, &data)
	if data == nil {
		data = map[string]any{}
	}
	return FyloRecord{Id: document.Id, Data: data}
}

// WithDocId answers the record's fields after its document id, as the
// JavaScript `{ docId, ...record }` spread built them: a stored docId field
// wins over the document id.
func (record FyloRecord) WithDocId() map[string]any {
	flattened := map[string]any{"docId": record.Id}
	for key, value := range record.Data {
		flattened[key] = value
	}
	return flattened
}
