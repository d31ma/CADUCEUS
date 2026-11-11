import (
	"bytes"
	"cmp"
	"encoding/json"

	"mesail.dev/packages/core"
)

// @Service
type DocumentsService struct {
	// @Needs
	dataSource DataSource
}

func (documents *DocumentsService) Exists(collection string) (bool, error) {
	exists, failure := documents.dataSource.CollectionExists(collection)
	return exists, documents.unavailable(failure)
}

func (documents *DocumentsService) Ensure(collection string) error {
	exists, failure := documents.dataSource.CollectionExists(collection)
	return documents.unavailable(cmp.Or(failure, documents.createUnless(exists, failure, collection)))
}

func (documents *DocumentsService) Insert(collection string, document any) (string, error) {
	fields, failure := documents.withoutId(document)
	id, insertFailure := documents.insertUnless(failure, collection, fields)
	return id, documents.unavailable(cmp.Or(failure, insertFailure))
}

func (documents *DocumentsService) Find(collection string, id string, target any) (bool, error) {
	content := json.RawMessage{}
	found, failure := documents.dataSource.FindById(collection, id, &content)
	decoded, decodeFailure := documents.decodeWithId(documents.readable(found, failure), content, id, target)
	return decoded, documents.unavailable(cmp.Or(failure, decodeFailure))
}

func (documents *DocumentsService) Replace(collection string, id string, document any) error {
	fields, failure := documents.withoutId(document)
	return documents.unavailable(cmp.Or(failure, documents.patchUnless(failure, collection, id, fields)))
}

func (documents *DocumentsService) Delete(collection string, id string) error {
	_, failure := documents.dataSource.DeleteById(collection, id)
	return documents.unavailable(failure)
}

func (documents *DocumentsService) All(collection string, where map[string]any, target any) error {
	rows, failure := documents.collect(collection, where)
	encoded, _ := json.Marshal(rows)
	return documents.unavailable(cmp.Or(failure, json.Unmarshal(encoded, target)))
}

func (documents *DocumentsService) collect(collection string, where map[string]any) ([]json.RawMessage, error) {
	rows, cursor, failure := documents.page(collection, where, "")
	for cursor != "" {
		more, next, pageFailure := documents.page(collection, where, cursor)
		rows, cursor, failure = append(rows, more...), next, pageFailure
	}
	return rows, failure
}

func (documents *DocumentsService) page(collection string, where map[string]any, cursor string) ([]json.RawMessage, string, error) {
	window, failure := documents.dataSource.Scroll(collection, where, cursor, documents.batch())
	rows := []json.RawMessage{}
	decodeFailure := window.Decode(&rows)
	failure = cmp.Or(failure, decodeFailure)
	return rows, documents.nextCursor(window, failure), failure
}

func (documents *DocumentsService) nextCursor(window DocumentWindow, failure error) string {
	if failure != nil {
		return ""
	}
	return window.NextCursor
}

func (documents *DocumentsService) withoutId(document any) (map[string]any, error) {
	encoded, failure := json.Marshal(document)
	fields := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	decodeFailure := decoder.Decode(&fields)
	delete(fields, documents.idField())
	return fields, cmp.Or(failure, decodeFailure)
}

func (documents *DocumentsService) readable(found bool, failure error) bool {
	return found && failure == nil
}

func (documents *DocumentsService) decodeWithId(readable bool, content json.RawMessage, id string, target any) (bool, error) {
	if !readable {
		return false, nil
	}
	fields := map[string]json.RawMessage{}
	failure := json.Unmarshal(content, &fields)
	fields[documents.idField()], _ = json.Marshal(id)
	encoded, _ := json.Marshal(fields)
	decodeFailure := cmp.Or(failure, json.Unmarshal(encoded, target))
	return decodeFailure == nil, decodeFailure
}

func (documents *DocumentsService) insertUnless(failure error, collection string, fields map[string]any) (string, error) {
	if failure != nil {
		return "", nil
	}
	return documents.dataSource.Insert(collection, fields)
}

func (documents *DocumentsService) patchUnless(failure error, collection string, id string, fields map[string]any) error {
	if failure != nil {
		return nil
	}
	_, patchFailure := documents.dataSource.Patch(collection, id, fields, nil)
	return patchFailure
}

func (documents *DocumentsService) createUnless(exists bool, failure error, collection string) error {
	if exists || failure != nil {
		return nil
	}
	return documents.dataSource.CreateCollection(collection)
}

func (documents *DocumentsService) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return &core.Unavailable{Message: "the message store is not available", Cause: failure}
}

func (documents *DocumentsService) idField() string {
	return "id"
}

func (documents *DocumentsService) batch() int {
	return 100
}
