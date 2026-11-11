package mailin

import (
	"encoding/json"
	"math"
)

type Storage struct {
	Documents Documents
	Pages     PageReader
}

func (storage Storage) Ensure(collection string) error {
	exists, err := storage.Documents.CollectionExists(collection)
	if err != nil {
		return Unavailable{Operation: "collection check"}
	}
	if exists {
		return nil
	}
	return storage.failure("create collection", storage.Documents.CreateCollection(collection))
}

func (storage Storage) Insert(collection string, document any) (string, error) {
	id, err := storage.Documents.Insert(collection, document)
	return id, storage.failure("insert", err)
}

func (storage Storage) Find(collection string, id string, target any) (bool, error) {
	found, err := storage.Documents.FindById(collection, id, target)
	return found, storage.failure("find", err)
}

func (storage Storage) Replace(collection string, id string, document any) error {
	_, err := storage.Documents.Patch(collection, id, document, nil)
	return storage.failure("patch", err)
}

func (storage Storage) All(collection string, where map[string]any, target any) error {
	return storage.First(collection, where, storage.Everything(), target)
}

func (storage Storage) First(collection string, where map[string]any, limit int, target any) error {
	rows, err := storage.rows(collection, where, limit)
	if err != nil {
		return Unavailable{Operation: "scroll"}
	}
	encoded, _ := json.Marshal(rows)
	return storage.failure("decode", json.Unmarshal(encoded, target))
}

func (Storage) Everything() int {
	return math.MaxInt
}

func (storage Storage) rows(collection string, where map[string]any, limit int) ([]json.RawMessage, error) {
	rows := []json.RawMessage{}
	cursor := ""
	for len(rows) < limit {
		page, err := storage.Pages(collection, where, cursor, min(storage.batch(), limit-len(rows)))
		if err != nil {
			return nil, err
		}
		more, err := storage.decoded(page.Content)
		if err != nil {
			return nil, err
		}
		rows = append(rows, more...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return rows[:min(limit, len(rows))], nil
}

func (Storage) decoded(content json.RawMessage) ([]json.RawMessage, error) {
	rows := []json.RawMessage{}
	if len(content) == 0 {
		return rows, nil
	}
	return rows, json.Unmarshal(content, &rows)
}

func (Storage) failure(operation string, err error) error {
	if err == nil {
		return nil
	}
	return Unavailable{Operation: operation}
}

func (Storage) batch() int {
	return 100
}
