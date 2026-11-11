package mailin

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"mesail.dev/packages/outbox"
)

type memoryDocuments struct {
	collections map[string][]string
	rows        map[string]map[string]map[string]any
	failOn      string
	badContent  bool
	failPagesIn string
	created     int
	pageReads   map[string]int
	patches     int
}

func newMemory() *memoryDocuments {
	return &memoryDocuments{collections: map[string][]string{}, rows: map[string]map[string]map[string]any{}, pageReads: map[string]int{}}
}

func (memory *memoryDocuments) failure(operation string) error {
	if memory.failOn == operation {
		return errors.New(operation + " failed")
	}
	return nil
}

func (memory *memoryDocuments) CollectionExists(name string) (bool, error) {
	_, found := memory.rows[name]
	return found, memory.failure("exists")
}

func (memory *memoryDocuments) CreateCollection(name string) error {
	if err := memory.failure("create"); err != nil {
		return err
	}
	memory.created++
	memory.rows[name] = map[string]map[string]any{}
	return nil
}

func (memory *memoryDocuments) Insert(collection string, document any) (string, error) {
	if err := memory.failure("insert"); err != nil {
		return "", err
	}
	id := "D" + strconv.Itoa(len(memory.collections[collection])+1)
	memory.collections[collection] = append(memory.collections[collection], id)
	memory.rows[collection][id] = memory.asMap(document)
	return id, nil
}

func (memory *memoryDocuments) FindById(collection string, id string, target any) (bool, error) {
	row, found := memory.rows[collection][id]
	raw, _ := json.Marshal(row)
	json.Unmarshal(raw, target)
	return found, memory.failure("find")
}

func (memory *memoryDocuments) Patch(collection string, id string, fields any, target any) (bool, error) {
	if err := memory.failure("patch"); err != nil {
		return false, err
	}
	memory.patches++
	for key, value := range memory.asMap(fields) {
		memory.rows[collection][id][key] = value
	}
	return true, nil
}

func (memory *memoryDocuments) Page(collection string, where map[string]any, cursor string, limit int) (Page, error) {
	if err := memory.failure("page"); err != nil {
		return Page{}, err
	}
	memory.pageReads[collection]++
	if collection == memory.failPagesIn {
		return Page{}, errors.New("page failed")
	}
	if memory.badContent {
		return Page{Content: json.RawMessage("{")}, nil
	}
	start, _ := strconv.Atoi(cursor)
	ids := memory.matching(collection, where)
	if start >= len(ids) {
		return Page{}, nil
	}
	row := map[string]any{}
	for key, value := range memory.rows[collection][ids[start]] {
		row[key] = value
	}
	row["id"] = ids[start]
	content, _ := json.Marshal([]map[string]any{row})
	return Page{Content: content, NextCursor: memory.next(start, len(ids))}, nil
}

func (memory *memoryDocuments) next(start int, total int) string {
	if start+1 >= total {
		return ""
	}
	return strconv.Itoa(start + 1)
}

func (memory *memoryDocuments) matching(collection string, where map[string]any) []string {
	ids := []string{}
	for _, id := range memory.collections[collection] {
		if memory.matches(memory.rows[collection][id], where) {
			ids = append(ids, id)
		}
	}
	return ids
}

func (memory *memoryDocuments) matches(row map[string]any, where map[string]any) bool {
	for key, value := range where {
		if fmt.Sprint(row[key]) != fmt.Sprint(value) {
			return false
		}
	}
	return true
}

func (memory *memoryDocuments) asMap(document any) map[string]any {
	raw, _ := json.Marshal(document)
	values := map[string]any{}
	json.Unmarshal(raw, &values)
	return values
}

func (memory *memoryDocuments) storage() Storage {
	return Storage{Documents: memory, Pages: memory.Page}
}

func (memory *memoryDocuments) registry() CollectionRegistry {
	return CollectionRegistry{Storage: memory.storage()}
}

func (memory *memoryDocuments) store() OutboxStore {
	return OutboxStore{Storage: memory.storage(), Registry: memory.registry()}
}

func testEnvironment(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func goodSettings() func(string) string {
	application := `{"yon":{"outbox":{"producer":"mail-in","subscribers":[{"name":"messaging","url":"http://127.0.0.1:9/events"}],"retry":{"maxAttempts":2}}}}`
	return testEnvironment(map[string]string{"YON_APPLICATION_JSON": application, "YON_SERVICE_TOKEN": "service-token"})
}

func fixedNow() time.Time {
	return time.Date(2026, 11, 10, 22, 0, 0, 0, time.UTC)
}

func testDesk(memory *memoryDocuments, pusher outbox.Pusher, environment func(string) string) OutboxDesk {
	return OutboxDesk{Store: memory.store(), Pusher: pusher, Settings: SettingsReader{Environment: environment}, Now: fixedNow}
}

func testInbox(memory *memoryDocuments, pusher outbox.Pusher, environment func(string) string) Inbox {
	settings := SettingsReader{Environment: environment}
	stager := EventStager{Settings: settings, Now: fixedNow, Random: zeroReader{}}
	return Inbox{Storage: memory.storage(), Registry: memory.registry(), Stager: stager, Outbox: testDesk(memory, pusher, environment), Now: fixedNow}
}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 7
	}
	return len(buffer), nil
}

type failingReader struct{}

func (failingReader) Read(buffer []byte) (int, error) {
	return 0, errors.New("no randomness")
}

func testEnvelope(domainId string, key string) Envelope {
	return Envelope{
		EnvelopeFrom: "dan@outside.test",
		Recipients:   []string{"ana@" + domainId},
		RawBlobKey:   key,
		Size:         64,
		Headers:      Headers{MessageId: "bb-1@outside.test", Subject: "Hello", From: Mailbox{Name: "Dan", Address: "dan@outside.test"}},
		AuthResults:  AuthResults{Spf: "pass", Dkim: "pass", Dmarc: "pass", DmarcPolicy: "none"},
	}
}

func check(t *testing.T, ok bool, message string) {
	t.Helper()
	if !ok {
		t.Fatal(message)
	}
}

func isUnavailable(err error, operation string) bool {
	unavailable := Unavailable{}
	return errors.As(err, &unavailable) && unavailable.Operation == operation
}

func isMissing(err error) bool {
	return errors.As(err, &Missing{})
}

func invalidField(err error) string {
	invalid := Invalid{}
	errors.As(err, &invalid)
	return invalid.Field
}
