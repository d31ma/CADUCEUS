import (
	"bytes"
	"encoding/json"
	"errors"
)

// FyloDocuments reads a FYLO answer of the form {"id": document, ...} into
// documents in the order FYLO listed them, which is the order routes answer.
type FyloDocuments struct{}

// Ordered answers the documents of one answer.
func (FyloDocuments) Ordered(raw json.RawMessage) ([]FyloDocument, error) {
	documents := []FyloDocument{}
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return documents, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, errors.New("FYLO answered documents that are not an object")
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		var data json.RawMessage
		if err := decoder.Decode(&data); err != nil {
			return nil, err
		}
		id, _ := key.(string)
		documents = append(documents, FyloDocument{Id: id, Data: data})
	}
	return documents, nil
}
