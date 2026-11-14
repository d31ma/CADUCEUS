package search

import (
	"encoding/json"
	"errors"

	"mesail.dev/packages/collections"
)

type DocumentStore struct {
	Documents Documents
	Known     collections.KnownCollections
}

func (store DocumentStore) Ensure(collection string) error {
	return store.unavailable(store.Known.Ensure(store.Documents, collection))
}

func (store DocumentStore) Insert(collection string, document any) (string, error) {
	id := ""
	err := store.Known.Write(store.Documents, collection, func() error {
		var failure error
		id, failure = store.Documents.Insert(collection, document)
		return failure
	})
	return id, store.unavailable(err)
}

func (store DocumentStore) Find(collection string, id string, target any) error {
	_, err := store.Documents.FindById(collection, id, target)
	return store.unavailable(err)
}

func (store DocumentStore) Replace(collection string, id string, document any) error {
	_, err := store.Documents.Patch(collection, id, document, nil)
	return store.unavailable(err)
}

func (store DocumentStore) All(collection string, where map[string]any, target any) error {
	rows := []json.RawMessage{}
	err := store.Known.Read(store.Documents, collection, func() error {
		var failure error
		rows, failure = store.rows(collection, where)
		return failure
	})
	if err != nil {
		return store.unavailable(err)
	}
	encoded, _ := json.Marshal(rows)
	return store.unavailable(json.Unmarshal(encoded, target))
}

func (store DocumentStore) rows(collection string, where map[string]any) ([]json.RawMessage, error) {
	rows, cursor, err := store.scroll(collection, where, "")
	for err == nil && cursor != "" {
		var page []json.RawMessage
		page, cursor, err = store.scroll(collection, where, cursor)
		rows = append(rows, page...)
	}
	return rows, err
}

func (store DocumentStore) scroll(collection string, where map[string]any, cursor string) ([]json.RawMessage, string, error) {
	answer, err := store.Documents.Command(store.scrollCommand(collection, where, cursor))
	window := Window{}
	encoded, _ := json.Marshal(answer)
	decoded := json.Unmarshal(encoded, &window)
	return window.Content, window.NextCursor, errors.Join(err, decoded)
}

func (store DocumentStore) scrollCommand(collection string, where map[string]any, cursor string) map[string]any {
	command := map[string]any{"operation": "document", "command": "scroll", "collection": collection, "where": where, "size": store.batch()}
	if cursor != "" {
		command["cursor"] = cursor
	}
	return command
}

func (DocumentStore) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return Unavailable{Message: "the search index is not available", Cause: failure}
}

func (DocumentStore) batch() int {
	return 100
}
