package mailout

import (
	"encoding/json"

	"mesail.dev/packages/collections"
)

type DocumentStore struct {
	Documents Documents
}

func (DocumentStore) pageSize() int { return 100 }

func (store DocumentStore) Ready(base string, domainId string) (string, error) {
	name := collections.CollectionName{}.ForDomain(base, domainId)
	return name, store.Documents.CreateCollection(name)
}

func (store DocumentStore) Page(collection string, where map[string]any, cursor string, size int) ([]json.RawMessage, string, error) {
	command := map[string]any{"operation": "document", "command": "scroll", "collection": collection, "where": where, "size": size}
	if cursor != "" {
		command["cursor"] = cursor
	}
	answer, err := store.Documents.Command(command)
	if err != nil {
		return nil, "", err
	}
	return store.window(answer)
}

func (store DocumentStore) All(collection string, where map[string]any) ([]json.RawMessage, error) {
	documents, cursor, err := store.Page(collection, where, "", store.pageSize())
	for err == nil && cursor != "" {
		var more []json.RawMessage
		more, cursor, err = store.Page(collection, where, cursor, store.pageSize())
		documents = append(documents, more...)
	}
	return documents, err
}

func (store DocumentStore) Exists(collection string, where map[string]any) (bool, error) {
	documents, _, err := store.Page(collection, where, "", 1)
	return len(documents) > 0, err
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
