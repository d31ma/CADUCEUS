import (
	"cmp"
	"encoding/json"
)

// @Service
type DocumentsService struct {
	// @Needs
	dataSource DataSource
}

func (documents *DocumentsService) Ensure(collection string) error {
	exists, failure := documents.dataSource.CollectionExists(collection)
	return cmp.Or(failure, documents.createUnless(exists, collection))
}

func (documents *DocumentsService) Drop(collection string) error {
	_, failure := documents.dataSource.DropCollection(collection)
	return failure
}

func (documents *DocumentsService) Insert(collection string, document any) (string, error) {
	ensureFailure := documents.Ensure(collection)
	id, failure := documents.dataSource.Insert(collection, document)
	return id, cmp.Or(ensureFailure, failure)
}

func (documents *DocumentsService) ById(collection string, id string, target any) (bool, error) {
	return documents.dataSource.FindById(collection, id, target)
}

func (documents *DocumentsService) Replace(collection string, id string, document any) error {
	_, failure := documents.dataSource.Patch(collection, id, document, nil)
	return failure
}

func (documents *DocumentsService) Delete(collection string, id string) error {
	_, failure := documents.dataSource.DeleteById(collection, id)
	return failure
}

func (documents *DocumentsService) page(request PageRequest, target any) (string, error) {
	window, failure := documents.dataSource.Scroll(request.Collection, request.Where, request.Cursor, request.Size)
	decodeFailure := window.Decode(target)
	return window.NextCursor, cmp.Or(failure, decodeFailure)
}

func (documents *DocumentsService) All(collection string, where map[string]any, target any) error {
	ensureFailure := documents.Ensure(collection)
	rows, failure := documents.collect(collection, where)
	encoded, _ := json.Marshal(rows)
	return cmp.Or(ensureFailure, failure, json.Unmarshal(encoded, target))
}

func (documents *DocumentsService) FindOne(collection string, where map[string]any, target any) (bool, error) {
	ensureFailure := documents.Ensure(collection)
	rows := []json.RawMessage{}
	_, failure := documents.page(PageRequest{Collection: collection, Where: where, Size: documents.batch()}, &rows)
	return documents.decodeFirst(rows, target), cmp.Or(ensureFailure, failure)
}

func (documents *DocumentsService) collect(collection string, where map[string]any) ([]json.RawMessage, error) {
	rows := []json.RawMessage{}
	cursor, failure := documents.page(PageRequest{Collection: collection, Where: where, Size: documents.batch()}, &rows)
	for cursor != "" {
		page := []json.RawMessage{}
		next, pageFailure := documents.page(PageRequest{Collection: collection, Where: where, Cursor: cursor, Size: documents.batch()}, &page)
		rows = append(rows, page...)
		cursor = next
		failure = cmp.Or(failure, pageFailure)
	}
	return rows, failure
}

func (documents *DocumentsService) decodeFirst(rows []json.RawMessage, target any) bool {
	if len(rows) == 0 {
		return false
	}
	return json.Unmarshal(rows[0], target) == nil
}

func (documents *DocumentsService) createUnless(exists bool, collection string) error {
	if exists {
		return nil
	}
	return documents.dataSource.CreateCollection(collection)
}

func (documents *DocumentsService) batch() int {
	return 100
}
