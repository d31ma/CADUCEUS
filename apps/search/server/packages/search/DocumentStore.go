package search

import (
	"encoding/json"
	"errors"
)

type DocumentStore struct {
	Documents Documents
}

func (store DocumentStore) Ensure(collection string) error {
	exists, err := store.Documents.CollectionExists(collection)
	if err != nil {
		return store.unavailable(err)
	}
	return store.createUnless(exists, collection)
}

func (store DocumentStore) Insert(collection string, document any) (string, error) {
	err := store.Ensure(collection)
	if err != nil {
		return "", err
	}
	id, err := store.Documents.Insert(collection, document)
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
	exists, err := store.Documents.CollectionExists(collection)
	if err != nil {
		return store.unavailable(err)
	}
	rows, err := store.rowsWhen(exists, collection, where)
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(rows)
	return store.unavailable(json.Unmarshal(encoded, target))
}

func (store DocumentStore) rowsWhen(exists bool, collection string, where map[string]any) ([]json.RawMessage, error) {
	if !exists {
		return []json.RawMessage{}, nil
	}
	return store.rows(collection, where)
}

func (store DocumentStore) rows(collection string, where map[string]any) ([]json.RawMessage, error) {
	rows, cursor, err := store.scroll(collection, where, "")
	for err == nil && cursor != "" {
		var page []json.RawMessage
		page, cursor, err = store.scroll(collection, where, cursor)
		rows = append(rows, page...)
	}
	return rows, store.unavailable(err)
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

func (store DocumentStore) createUnless(exists bool, collection string) error {
	if exists {
		return nil
	}
	return store.unavailable(store.Documents.CreateCollection(collection))
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
