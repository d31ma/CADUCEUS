package mailin

import (
	"encoding/json"
	"math"

	"mesail.dev/packages/collections"
)

type Storage struct {
	Documents Documents
	Pages     PageReader
	Known     collections.KnownCollections
}

func (storage Storage) Ensure(collection string) error {
	_, err := storage.Exists(collection)
	if err != nil {
		return err
	}
	return storage.failure("create collection", storage.Known.Ensure(storage.Documents, collection))
}

func (storage Storage) Exists(collection string) (bool, error) {
	exists, err := storage.Known.Exists(storage.Documents, collection)
	return exists, storage.failure("collection check", err)
}

func (storage Storage) Insert(collection string, document any) (string, error) {
	id := ""
	err := storage.Known.Write(storage.Documents, collection, func() error {
		var failure error
		id, failure = storage.Documents.Insert(collection, document)
		return failure
	})
	return id, storage.failure("insert", err)
}

func (storage Storage) Find(collection string, id string, target any) (bool, error) {
	found, err := storage.Documents.FindById(collection, id, target)
	return found, storage.failure("find", storage.Known.ReadFailure(storage.Documents, collection, err))
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
	err = storage.Known.ReadFailure(storage.Documents, collection, err)
	if err != nil {
		return Unavailable{Operation: "scroll"}
	}
	encoded, _ := json.Marshal(rows)
	return storage.failure("decode", json.Unmarshal(encoded, target))
}

func (Storage) Where(filter any) map[string]any {
	encoded, _ := json.Marshal(filter)
	where := map[string]any{}
	json.Unmarshal(encoded, &where)
	return where
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
