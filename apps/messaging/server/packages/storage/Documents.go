package storage

import (
	"bytes"
	"cmp"
	"encoding/json"

	"mesail.dev/packages/core"
)

type Documents struct {
	Source DataSource
	Scroll Scroll
}

func (documents Documents) Exists(collection string) (bool, error) {
	exists, failure := documents.Source.CollectionExists(collection)
	return exists, documents.unavailable(failure)
}

func (documents Documents) Ensure(collection string) error {
	exists, failure := documents.Source.CollectionExists(collection)
	return documents.unavailable(cmp.Or(failure, documents.createUnless(exists, failure, collection)))
}

func (documents Documents) Insert(collection string, document any) (string, error) {
	fields, failure := documents.withoutId(document)
	id, insertFailure := documents.insertUnless(failure, collection, fields)
	return id, documents.unavailable(cmp.Or(failure, insertFailure))
}

func (documents Documents) Find(collection string, id string, target any) (bool, error) {
	content := json.RawMessage{}
	found, failure := documents.Source.FindById(collection, id, &content)
	decoded, decodeFailure := documents.decodeWithId(documents.readable(found, failure), content, id, target)
	return decoded, documents.unavailable(cmp.Or(failure, decodeFailure))
}

func (documents Documents) Replace(collection string, id string, document any) error {
	fields, failure := documents.withoutId(document)
	return documents.unavailable(cmp.Or(failure, documents.patchUnless(failure, collection, id, fields)))
}

func (documents Documents) Delete(collection string, id string) error {
	_, failure := documents.Source.DeleteById(collection, id)
	return documents.unavailable(failure)
}

func (documents Documents) All(collection string, where map[string]any, target any) error {
	rows, failure := documents.collect(collection, where)
	encoded, _ := json.Marshal(rows)
	return documents.unavailable(cmp.Or(failure, json.Unmarshal(encoded, target)))
}

func (documents Documents) collect(collection string, where map[string]any) ([]json.RawMessage, error) {
	rows, cursor, failure := documents.page(collection, where, "")
	for cursor != "" {
		more, next, pageFailure := documents.page(collection, where, cursor)
		rows, cursor, failure = append(rows, more...), next, pageFailure
	}
	return rows, failure
}

func (documents Documents) page(collection string, where map[string]any, cursor string) ([]json.RawMessage, string, error) {
	window, failure := documents.Scroll(collection, where, cursor, documents.batch())
	rows := []json.RawMessage{}
	decodeFailure := json.Unmarshal(window.Content, &rows)
	failure = cmp.Or(failure, decodeFailure)
	return rows, documents.nextCursor(window, failure), failure
}

func (Documents) nextCursor(window Window, failure error) string {
	if failure != nil {
		return ""
	}
	return window.NextCursor
}

func (documents Documents) withoutId(document any) (map[string]any, error) {
	encoded, failure := json.Marshal(document)
	fields := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	decodeFailure := decoder.Decode(&fields)
	delete(fields, documents.idField())
	return fields, cmp.Or(failure, decodeFailure)
}

func (Documents) readable(found bool, failure error) bool {
	return found && failure == nil
}

func (documents Documents) decodeWithId(readable bool, content json.RawMessage, id string, target any) (bool, error) {
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

func (documents Documents) insertUnless(failure error, collection string, fields map[string]any) (string, error) {
	if failure != nil {
		return "", nil
	}
	return documents.Source.Insert(collection, fields)
}

func (documents Documents) patchUnless(failure error, collection string, id string, fields map[string]any) error {
	if failure != nil {
		return nil
	}
	_, patchFailure := documents.Source.Patch(collection, id, fields, nil)
	return patchFailure
}

func (documents Documents) createUnless(exists bool, failure error, collection string) error {
	if exists || failure != nil {
		return nil
	}
	return documents.Source.CreateCollection(collection)
}

func (Documents) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return &core.Unavailable{Message: "the message store is not available", Cause: failure}
}

func (Documents) idField() string {
	return "id"
}

func (Documents) batch() int {
	return 100
}
