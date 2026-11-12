package mailin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
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
	return testEnvironment(map[string]string{"YON_APPLICATION_JSON": application})
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
	store := newBlobs()
	store.fallback = []byte(textMessage())
	return Inbox{Storage: memory.storage(), Registry: memory.registry(), Stager: stager, Outbox: testDesk(memory, pusher, environment), Contents: testShelf(memory, store), Now: fixedNow}
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

type memoryBlobs struct {
	objects  map[string][]byte
	fallback []byte
	failOn   string
	signed   []blobs.DownloadTerms
}

func newBlobs() *memoryBlobs {
	return &memoryBlobs{objects: map[string][]byte{}}
}

func (memory *memoryBlobs) failure(operation string) error {
	if memory.failOn == operation {
		return errors.New(operation + " failed")
	}
	return nil
}

func (memory *memoryBlobs) SignUpload(key blobs.Key, upload blobs.UploadTerms, expiresAt time.Time) (string, error) {
	return "", errors.New("not used")
}

func (memory *memoryBlobs) SignDownload(key blobs.Key, download blobs.DownloadTerms, expiresAt time.Time) (string, error) {
	memory.signed = append(memory.signed, download)
	return "https://blobs.test/" + key.Path() + "?file=" + download.FileName, memory.failure("sign")
}

func (memory *memoryBlobs) Read(key blobs.Key) (io.ReadCloser, error) {
	content, found := memory.objects[key.Path()]
	if !found && memory.fallback == nil {
		return nil, blobs.NotFound{Path: key.Path()}
	}
	if !found {
		content = memory.fallback
	}
	return io.NopCloser(bytes.NewReader(content)), memory.failure("read")
}

func (memory *memoryBlobs) Write(key blobs.Key, body io.Reader, size int64) error {
	if err := memory.failure("write"); err != nil {
		return err
	}
	content, _ := io.ReadAll(body)
	memory.objects[key.Path()] = content
	return nil
}

func (memory *memoryBlobs) Copy(from blobs.Key, to blobs.Key) error {
	if err := memory.failure("copy"); err != nil {
		return err
	}
	content, found := memory.objects[from.Path()]
	if !found {
		return blobs.NotFound{Path: from.Path()}
	}
	memory.objects[to.Path()] = content
	return nil
}

func (memory *memoryBlobs) Delete(key blobs.Key) error {
	delete(memory.objects, key.Path())
	return nil
}

type fakeDomains struct {
	domains map[string]Domain
	failure error
	asked   []string
}

func (fake *fakeDomains) Domain(domainId string) (Domain, bool, error) {
	fake.asked = append(fake.asked, domainId)
	domain, found := fake.domains[domainId]
	return domain, found, fake.failure
}

type fakeCallers struct {
	domainId string
}

func (fake fakeCallers) Identify(authorization string) (oidc.Caller, error) {
	if authorization == "" {
		return oidc.Caller{}, errors.New("no token")
	}
	return oidc.Caller{DomainId: fake.domainId, Roles: []string{"Service"}}, nil
}

func activeDomains(names ...string) *fakeDomains {
	domains := map[string]Domain{}
	for _, name := range names {
		domains[name] = Domain{DomainId: name, Name: name, State: "active"}
	}
	return &fakeDomains{domains: domains}
}

func handOffInbox(memory *memoryDocuments, store *memoryBlobs, domains Domains) Inbox {
	inbox := testInbox(memory, outbox.NewRecordingPusher(), goodSettings())
	inbox.HandOff = HandOff{Domains: domains, Blobs: store}
	inbox.Contents = testShelf(memory, store)
	return inbox
}

func handOffEnvelope(senderKey string) Envelope {
	envelope := testEnvelope("example.com", senderKey)
	envelope.EnvelopeFrom = "bob@sender.test"
	envelope.Headers.From = Mailbox{Name: "Bob", Address: "bob@sender.test"}
	envelope.AuthResults = AuthResults{Handoff: true}
	return envelope
}

func testMessageDesk(memory *memoryDocuments, store *memoryBlobs, domainId string) MessageDesk {
	storage := memory.storage()
	return MessageDesk{Callers: fakeCallers{domainId: domainId}, Book: InboundBook{Storage: storage}, Contents: testShelf(memory, store), Blobs: store, LinkLife: 5 * time.Minute, Now: fixedNow}
}

func acceptedMessage(t *testing.T, memory *memoryDocuments, store *memoryBlobs, key string, raw string) string {
	store.objects[key] = []byte(raw)
	inbox := testInbox(memory, outbox.NewRecordingPusher(), goodSettings())
	inbox.Contents = testShelf(memory, store)
	inboundId, err := inbox.Accept(testEnvelope("example.com", key))
	check(t, err == nil, "accept failed")
	return inboundId
}
