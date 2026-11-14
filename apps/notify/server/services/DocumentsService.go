import (
	"cmp"
	"encoding/json"

	"mesail.dev/packages/collections"
)

// @Service
type DocumentsService struct {
	// @Needs
	dataSource DataSource
	known      collections.KnownCollections
}

func (documents *DocumentsService) Find(collection string, id string, target any) (bool, error) {
	found := false
	failure := documents.known.Read(documents.dataSource, collection, func() error {
		var findFailure error
		found, findFailure = documents.dataSource.FindById(collection, id, target)
		return findFailure
	})
	return found, documents.unavailable("find", failure)
}

func (documents *DocumentsService) Patch(collection string, id string, fields any) error {
	_, failure := documents.dataSource.Patch(collection, id, fields, nil)
	return documents.unavailable("patch", failure)
}

func (documents *DocumentsService) Delete(collection string, id string) error {
	_, failure := documents.dataSource.DeleteById(collection, id)
	return documents.unavailable("delete", failure)
}

func (documents *DocumentsService) Matching(collection string, where map[string]any, limit int, target any) error {
	rows := []json.RawMessage{}
	failure := documents.known.Read(documents.dataSource, collection, func() error {
		var readFailure error
		rows, readFailure = documents.collect(nil, collection, where, limit)
		return readFailure
	})
	decodeFailure := documents.decode(rows, target)
	return cmp.Or(documents.unavailable("read", failure), documents.unavailable("decode", decodeFailure))
}

func (documents *DocumentsService) InsertIfAbsent(collection string, where map[string]any, document any) (string, error) {
	existing := []DocumentId{}
	failure := documents.Matching(collection, where, 1, &existing)
	return documents.insertWhenAbsent(failure, existing, collection, document)
}

func (documents *DocumentsService) Upsert(collection string, where map[string]any, document any, patch any) error {
	existing := []DocumentId{}
	failure := documents.Matching(collection, where, 1, &existing)
	return documents.writeUnless(failure, collection, documents.firstId(existing), documents.choice(existing, document, patch))
}

func (documents *DocumentsService) insert(collection string, document any) (string, error) {
	id := ""
	failure := documents.known.Write(documents.dataSource, collection, func() error {
		var insertFailure error
		id, insertFailure = documents.dataSource.Insert(collection, document)
		return insertFailure
	})
	return id, documents.unavailable("insert", failure)
}

func (documents *DocumentsService) collect(failure error, collection string, where map[string]any, limit int) ([]json.RawMessage, error) {
	rows := []json.RawMessage{}
	cursor := ""
	for reading := failure == nil; reading; reading = documents.readMore(cursor, failure, len(rows), limit) {
		more, next, problem := documents.page(collection, where, cursor, limit-len(rows))
		rows, cursor, failure = append(rows, more...), next, problem
	}
	return rows[:min(len(rows), limit)], failure
}

func (documents *DocumentsService) readMore(cursor string, failure error, count int, limit int) bool {
	if cursor == "" || failure != nil || count >= limit {
		return false
	}
	return true
}

func (documents *DocumentsService) page(collection string, where map[string]any, cursor string, wanted int) ([]json.RawMessage, string, error) {
	window, failure := documents.dataSource.Scroll(collection, where, cursor, min(wanted, documents.pageSize()))
	rows := []json.RawMessage{}
	decodeFailure := json.Unmarshal(documents.content(window), &rows)
	return rows, window.NextCursor, documents.unavailable("scroll", cmp.Or(failure, decodeFailure))
}

func (documents *DocumentsService) content(window DocumentWindow) json.RawMessage {
	if len(window.Content) == 0 {
		return json.RawMessage("[]")
	}
	return window.Content
}

func (documents *DocumentsService) decode(rows []json.RawMessage, target any) error {
	encoded, _ := json.Marshal(rows)
	return json.Unmarshal(encoded, target)
}

func (documents *DocumentsService) insertWhenAbsent(failure error, existing []DocumentId, collection string, document any) (string, error) {
	if failure != nil || len(existing) > 0 {
		return documents.firstId(existing), failure
	}
	return documents.insert(collection, document)
}

func (documents *DocumentsService) choice(existing []DocumentId, document any, patch any) any {
	if len(existing) > 0 {
		return patch
	}
	return document
}

func (documents *DocumentsService) writeUnless(failure error, collection string, id string, value any) error {
	switch {
	case failure != nil:
		return failure
	case id == "":
		_, insertFailure := documents.insert(collection, value)
		return insertFailure
	}
	return documents.Patch(collection, id, value)
}

func (documents *DocumentsService) firstId(existing []DocumentId) string {
	return append(existing, DocumentId{})[0].Id
}

func (documents *DocumentsService) unavailable(operation string, failure error) error {
	if failure == nil {
		return nil
	}
	return StorageUnavailable{}.Store(operation)
}

func (documents *DocumentsService) pageSize() int {
	return 100
}
