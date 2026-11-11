package search

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"

	"mesail.dev/packages/oidc"
)

var errFakeStore = errors.New("the fake store failed")

type fakeRow struct {
	id     string
	fields map[string]json.RawMessage
}

type fakeDocuments struct {
	exists   map[string]bool
	rows     map[string][]*fakeRow
	failures map[string]int
	commands []map[string]any
	answer   any
	nextId   int
}

func newFakeDocuments() *fakeDocuments {
	return &fakeDocuments{exists: map[string]bool{}, rows: map[string][]*fakeRow{}, failures: map[string]int{}}
}

func (documents *fakeDocuments) failNext(method string) {
	documents.failAt(method, 0)
}

func (documents *fakeDocuments) failAt(method string, skip int) {
	documents.failures[method] = skip
}

func (documents *fakeDocuments) failure(method string) error {
	skip, armed := documents.failures[method]
	if !armed {
		return nil
	}
	if skip > 0 {
		documents.failures[method] = skip - 1
		return nil
	}
	delete(documents.failures, method)
	return errFakeStore
}

func (documents *fakeDocuments) CollectionExists(name string) (bool, error) {
	if err := documents.failure("CollectionExists"); err != nil {
		return false, err
	}
	return documents.exists[name], nil
}

func (documents *fakeDocuments) CreateCollection(name string) error {
	if err := documents.failure("CreateCollection"); err != nil {
		return err
	}
	documents.exists[name] = true
	return nil
}

func (documents *fakeDocuments) Insert(collection string, document any) (string, error) {
	if err := documents.failure("Insert"); err != nil {
		return "", err
	}
	if !documents.exists[collection] {
		return "", errors.New("no collection " + collection)
	}
	fields, err := fakeFields(document)
	if err != nil {
		return "", err
	}
	documents.nextId++
	id := "doc_" + strconv.Itoa(documents.nextId)
	documents.rows[collection] = append(documents.rows[collection], &fakeRow{id: id, fields: fields})
	return id, nil
}

func (documents *fakeDocuments) FindById(collection string, id string, target any) (bool, error) {
	if err := documents.failure("FindById"); err != nil {
		return false, err
	}
	row := documents.row(collection, id)
	if row == nil {
		return false, nil
	}
	encoded, _ := json.Marshal(row.fields)
	return true, json.Unmarshal(encoded, target)
}

func (documents *fakeDocuments) Patch(collection string, id string, fields any, target any) (bool, error) {
	if err := documents.failure("Patch"); err != nil {
		return false, err
	}
	row := documents.row(collection, id)
	if row == nil {
		return false, nil
	}
	patch, err := fakeFields(fields)
	if err != nil {
		return false, err
	}
	for name, value := range patch {
		row.fields[name] = value
	}
	return true, nil
}

func (documents *fakeDocuments) Command(command map[string]any) (any, error) {
	documents.commands = append(documents.commands, command)
	if err := documents.failure("Command"); err != nil {
		return nil, err
	}
	if documents.answer != nil {
		return documents.answer, nil
	}
	if command["operation"] != "document" || command["command"] != "scroll" {
		return nil, errors.New("the fake only knows the document scroll command")
	}
	return documents.scroll(command)
}

func (documents *fakeDocuments) scroll(command map[string]any) (any, error) {
	collection, _ := command["collection"].(string)
	where, _ := command["where"].(map[string]any)
	size, _ := command["size"].(int)
	cursor, _ := command["cursor"].(string)
	start := 0
	if cursor != "" {
		start, _ = strconv.Atoi(cursor)
	}
	matching := []*fakeRow{}
	for _, row := range documents.rows[collection] {
		if fakeMatches(row, where) {
			matching = append(matching, row)
		}
	}
	end := min(len(matching), start+size)
	content := []any{}
	for _, row := range matching[start:end] {
		encoded, _ := json.Marshal(row.fields)
		decoded := map[string]any{}
		json.Unmarshal(encoded, &decoded)
		content = append(content, decoded)
	}
	next := ""
	if end < len(matching) {
		next = strconv.Itoa(end)
	}
	return map[string]any{"content": content, "nextCursor": next}, nil
}

func (documents *fakeDocuments) row(collection string, id string) *fakeRow {
	for _, row := range documents.rows[collection] {
		if row.id == id {
			return row
		}
	}
	return nil
}

func (documents *fakeDocuments) put(collection string, document any) string {
	documents.exists[collection] = true
	id, err := documents.Insert(collection, document)
	if err != nil {
		panic(err)
	}
	return id
}

func fakeFields(document any) (map[string]json.RawMessage, error) {
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	fields := map[string]json.RawMessage{}
	return fields, json.Unmarshal(encoded, &fields)
}

func fakeMatches(row *fakeRow, where map[string]any) bool {
	for name, wanted := range where {
		encoded, _ := json.Marshal(wanted)
		if !bytes.Equal(encoded, row.fields[name]) {
			return false
		}
	}
	return true
}

type fakeReader struct {
	readable map[string][]string
	err      error
}

func (reader *fakeReader) Readable(authorization string) (map[string]bool, error) {
	if reader.err != nil {
		return nil, reader.err
	}
	readable := map[string]bool{}
	for _, conversationId := range reader.readable[authorization] {
		readable[conversationId] = true
	}
	return readable, nil
}

type fakeIdentifier struct {
	callers map[string]oidc.Caller
}

func (identifier *fakeIdentifier) Identify(authorization string) (oidc.Caller, error) {
	caller, known := identifier.callers[authorization]
	if !known {
		return oidc.Caller{}, errors.New("unknown token")
	}
	return caller, nil
}
