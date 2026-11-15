package mailout

import (
	"encoding/json"

	"mesail.dev/packages/collections"
)

type DocumentStore struct {
	Documents Documents
	Known     collections.KnownCollections
}

func (DocumentStore) pageSize() int { return 100 }

func (DocumentStore) Name(base string, domainId string) string {
	return collections.CollectionName{}.ForDomain(base, domainId)
}

func (store DocumentStore) Insert(collection string, document any) (string, error) {
	id := ""
	err := store.Known.Write(store.Documents, collection, func() error {
		var failure error
		id, failure = store.Documents.Insert(collection, document)
		return failure
	})
	return id, err
}

func (store DocumentStore) Page(collection string, where any, cursor string, size int) ([]json.RawMessage, string, error) {
	documents, next := []json.RawMessage{}, ""
	err := store.Known.Read(store.Documents, collection, func() error {
		var failure error
		documents, next, failure = store.read(collection, where, cursor, size)
		return failure
	})
	return documents, next, err
}

func (store DocumentStore) All(collection string, where any) ([]json.RawMessage, error) {
	documents, cursor, err := store.Page(collection, where, "", store.pageSize())
	if err != nil {
		return documents, err
	}
	return store.remaining(collection, where, documents, cursor)
}

func (store DocumentStore) Exists(collection string, where any) (bool, error) {
	documents, _, err := store.Page(collection, where, "", 1)
	return len(documents) > 0, err
}

func (store DocumentStore) read(collection string, where any, cursor string, size int) ([]json.RawMessage, string, error) {
	answer, err := store.Documents.Command(store.scroll(collection, where, cursor, size))
	if err != nil {
		return nil, "", err
	}
	return store.window(answer)
}

func (DocumentStore) scroll(collection string, where any, cursor string, size int) map[string]any {
	command := map[string]any{"operation": "document", "command": "scroll", "collection": collection, "where": where, "size": size}
	if cursor != "" {
		command["cursor"] = cursor
	}
	return command
}

func (store DocumentStore) remaining(collection string, where any, documents []json.RawMessage, cursor string) ([]json.RawMessage, error) {
	for cursor != "" {
		more, next, err := store.Page(collection, where, cursor, store.pageSize())
		documents, cursor = append(documents, more...), next
		if err != nil {
			return documents, err
		}
	}
	return documents, nil
}

func (DocumentStore) window(answer any) ([]json.RawMessage, string, error) {
	raw, err := json.Marshal(answer)
	if err != nil {
		return nil, "", err
	}
	window := Window{}
	err = json.Unmarshal(raw, &window)
	if err != nil {
		return nil, "", err
	}
	documents := []json.RawMessage{}
	err = json.Unmarshal(window.Content, &documents)
	return documents, window.NextCursor, err
}
