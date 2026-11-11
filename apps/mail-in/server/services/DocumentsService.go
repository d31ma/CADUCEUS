import (
	"encoding/json"
	"errors"
)

// @Service
type DocumentsService struct {
	// @Needs
	dataSource DataSource
}

func (documents *DocumentsService) Ensure(collection string) error {
	exists, err := documents.dataSource.CollectionExists(collection)
	if err != nil {
		return documents.unavailable("collection check", err)
	}
	return documents.unavailable("create collection", documents.createUnless(exists, collection))
}

func (documents *DocumentsService) Insert(collection string, document any) (string, error) {
	id, err := documents.dataSource.Insert(collection, document)
	return id, documents.unavailable("insert", err)
}

func (documents *DocumentsService) Find(collection string, id string, target any) (bool, error) {
	found, err := documents.dataSource.FindById(collection, id, target)
	return found, documents.unavailable("find", err)
}

func (documents *DocumentsService) Replace(collection string, id string, document any) error {
	_, err := documents.dataSource.Patch(collection, id, document, nil)
	return documents.unavailable("patch", err)
}

func (documents *DocumentsService) All(collection string, where map[string]any, target any) error {
	rows, err := documents.collect(collection, where)
	if err != nil {
		return documents.unavailable("scroll", err)
	}
	encoded, _ := json.Marshal(rows)
	return documents.unavailable("decode", json.Unmarshal(encoded, target))
}

func (documents *DocumentsService) collect(collection string, where map[string]any) ([]json.RawMessage, error) {
	rows, cursor, err := documents.page(collection, where, "")
	for cursor != "" {
		more, next, failure := documents.page(collection, where, cursor)
		rows, cursor, err = append(rows, more...), next, failure
	}
	return rows, err
}

func (documents *DocumentsService) page(collection string, where map[string]any, cursor string) ([]json.RawMessage, string, error) {
	window, err := documents.dataSource.Scroll(collection, where, cursor, documents.batch())
	rows := []json.RawMessage{}
	decodeError := json.Unmarshal(documents.content(window), &rows)
	return rows, window.NextCursor, errors.Join(err, decodeError)
}

func (documents *DocumentsService) content(window DocumentWindow) json.RawMessage {
	if len(window.Content) == 0 {
		return json.RawMessage("[]")
	}
	return window.Content
}

func (documents *DocumentsService) createUnless(exists bool, collection string) error {
	if exists {
		return nil
	}
	return documents.dataSource.CreateCollection(collection)
}

func (documents *DocumentsService) unavailable(operation string, failure error) error {
	if failure == nil {
		return nil
	}
	return &StorageUnavailable{Operation: operation}
}

func (documents *DocumentsService) batch() int {
	return 100
}
