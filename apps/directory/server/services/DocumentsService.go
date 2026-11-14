import (
	"cmp"
	"encoding/json"

	"mesail.dev/packages/collections"
)

// @Service
type DocumentsService struct {
	// @Needs
	dataSource DataSource
	known      collections.KnownCollections
}

func (documents *DocumentsService) Ensure(collection string) error {
	return documents.known.Ensure(documents.dataSource, collection)
}

func (documents *DocumentsService) Drop(collection string) error {
	documents.known.Forget(collection)
	_, failure := documents.dataSource.DropCollection(collection)
	return failure
}

func (documents *DocumentsService) Insert(collection string, document any) (string, error) {
	id := ""
	failure := documents.known.Write(documents.dataSource, collection, func() error {
		var insertFailure error
		id, insertFailure = documents.dataSource.Insert(collection, document)
		return insertFailure
	})
	return id, failure
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
	rows := []json.RawMessage{}
	failure := documents.known.Read(documents.dataSource, collection, func() error {
		var readFailure error
		rows, readFailure = documents.collect(collection, where)
		return readFailure
	})
	encoded, _ := json.Marshal(rows)
	return cmp.Or(failure, json.Unmarshal(encoded, target))
}

func (documents *DocumentsService) FindOne(collection string, where map[string]any, target any) (bool, error) {
	rows := []json.RawMessage{}
	failure := documents.known.Read(documents.dataSource, collection, func() error {
		_, readFailure := documents.page(PageRequest{Collection: collection, Where: where, Size: documents.batch()}, &rows)
		return readFailure
	})
	return documents.decodeFirst(rows, target), failure
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

func (documents *DocumentsService) batch() int {
	return 100
}
