package realtime

import "encoding/json"

type DocumentShelf struct {
	Storage Storage
}

func (shelf DocumentShelf) Open(collection string) error {
	exists, err := shelf.Storage.Documents.CollectionExists(collection)
	if err != nil {
		return err
	}
	return shelf.create(collection, exists)
}

func (shelf DocumentShelf) First(collection string, where map[string]any, target any) (bool, error) {
	found := []json.RawMessage{}
	_, err := shelf.Storage.Page(Query{Collection: collection, Where: where, Size: 1}, &found)
	if err != nil {
		return false, err
	}
	return len(found) > 0, shelf.decodeFirst(found, target)
}

func (shelf DocumentShelf) Put(collection string, id string, document any) error {
	return shelf.write(collection, id, document)
}

func (shelf DocumentShelf) write(collection string, id string, document any) error {
	if id == "" {
		_, err := shelf.Storage.Documents.Insert(collection, document)
		return err
	}
	_, err := shelf.Storage.Documents.Patch(collection, id, document, nil)
	return err
}

func (shelf DocumentShelf) create(collection string, exists bool) error {
	if exists {
		return nil
	}
	return shelf.Storage.Documents.CreateCollection(collection)
}

func (DocumentShelf) decodeFirst(found []json.RawMessage, target any) error {
	if len(found) == 0 {
		return nil
	}
	return json.Unmarshal(found[0], target)
}
