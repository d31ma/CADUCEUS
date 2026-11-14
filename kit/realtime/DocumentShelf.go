package realtime

import (
	"encoding/json"

	"mesail.dev/packages/collections"
)

type DocumentShelf struct {
	Storage Storage
	Known   collections.KnownCollections
}

func (shelf DocumentShelf) Open(collection string) error {
	return shelf.Known.Ensure(shelf.Storage.Documents, collection)
}

func (shelf DocumentShelf) First(collection string, where map[string]any, target any) (bool, error) {
	found := []json.RawMessage{}
	_, err := shelf.Storage.Page(Query{Collection: collection, Where: where, Size: 1}, &found)
	err = shelf.Known.ReadFailure(shelf.Storage.Documents, collection, err)
	if err != nil {
		return false, err
	}
	return len(found) > 0, shelf.decodeFirst(found, target)
}

func (shelf DocumentShelf) Every(collection string, where map[string]any, target any) error {
	found, err := shelf.pages(collection, where)
	if err != nil {
		return err
	}
	return shelf.decodeAll(found, target)
}

func (shelf DocumentShelf) Put(collection string, id string, document any) error {
	return shelf.Known.Write(shelf.Storage.Documents, collection, func() error {
		return shelf.write(collection, id, document)
	})
}

func (shelf DocumentShelf) write(collection string, id string, document any) error {
	if id == "" {
		_, err := shelf.Storage.Documents.Insert(collection, document)
		return err
	}
	_, err := shelf.Storage.Documents.Patch(collection, id, document, nil)
	return err
}

func (DocumentShelf) decodeFirst(found []json.RawMessage, target any) error {
	if len(found) == 0 {
		return nil
	}
	return json.Unmarshal(found[0], target)
}

func (shelf DocumentShelf) pages(collection string, where map[string]any) ([]json.RawMessage, error) {
	found, cursor, more := []json.RawMessage{}, "", true
	for pages := 0; more && pages < shelf.maximumPages(); pages++ {
		page := []json.RawMessage{}
		next, err := shelf.Storage.Page(Query{Collection: collection, Where: where, Cursor: cursor, Size: shelf.pageSize()}, &page)
		if err != nil {
			return nil, err
		}
		found, cursor, more = append(found, page...), next, next != ""
	}
	return found, nil
}

func (DocumentShelf) decodeAll(found []json.RawMessage, target any) error {
	encoded, err := json.Marshal(found)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

func (DocumentShelf) pageSize() int {
	return 100
}

func (DocumentShelf) maximumPages() int {
	return 1000
}
