package backups

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/collections"
	"mesail.dev/packages/oidc"
)

type memoryDocuments struct {
	collections    map[string]map[string]map[string]any
	failOn         string
	next           int
	emptyFirstPage bool
}

func newMemory() *memoryDocuments {
	return &memoryDocuments{collections: map[string]map[string]map[string]any{}}
}

func (memory *memoryDocuments) failure(operation string) error {
	if memory.failOn == operation {
		return errors.New(operation + " failed")
	}
	return nil
}

func (memory *memoryDocuments) CollectionExists(name string) (bool, error) {
	_, exists := memory.collections[name]
	return exists, memory.failure("exists")
}

func (memory *memoryDocuments) CreateCollection(name string) error {
	memory.collections[name] = map[string]map[string]any{}
	return memory.failure("create")
}

func (memory *memoryDocuments) Insert(collection string, document any) (string, error) {
	if err := memory.failure("insert"); err != nil {
		return "", err
	}
	if err := memory.objectArrays(document); err != nil {
		return "", err
	}
	memory.next++
	id := "doc" + strconv.Itoa(memory.next)
	memory.collections[collection][id] = memory.fields(document, id)
	return id, nil
}

func (memory *memoryDocuments) Patch(collection string, id string, fields any, target any) (bool, error) {
	if err := memory.failure("patch"); err != nil {
		return false, err
	}
	if err := memory.objectArrays(fields); err != nil {
		return false, err
	}
	memory.collections[collection][id] = memory.fields(fields, id)
	return true, nil
}

func (memory *memoryDocuments) DeleteById(collection string, id string) (bool, error) {
	if err := memory.failure("delete"); err != nil {
		return false, err
	}
	delete(memory.collections[collection], id)
	return true, nil
}

func (memory *memoryDocuments) Scroll(collection string, where map[string]any, cursor string, size int) (Window, error) {
	if err := memory.failure("scroll"); err != nil {
		return Window{}, err
	}
	if memory.emptyFirstPage && cursor == "" {
		return Window{NextCursor: "page-2"}, nil
	}
	found := []map[string]any{}
	for _, document := range memory.collections[collection] {
		if memory.matches(document, where) && len(found) < size {
			found = append(found, document)
		}
	}
	content, err := json.Marshal(found)
	return Window{Content: content}, err
}

func (memory *memoryDocuments) matches(document map[string]any, where map[string]any) bool {
	for field, value := range where {
		if document[field] != value {
			return false
		}
	}
	return true
}

func (memory *memoryDocuments) fields(document any, id string) map[string]any {
	text, _ := json.Marshal(document)
	fields := map[string]any{}
	json.Unmarshal(text, &fields)
	fields["id"] = id
	return fields
}

func (memory *memoryDocuments) objectArrays(document any) error {
	text, _ := json.Marshal(document)
	if strings.Contains(string(text), "[{") {
		return errors.New("EARRAYOFOBJECTS")
	}
	return nil
}

func (memory *memoryDocuments) count(collection string) int {
	return len(memory.collections[collection])
}

type memoryStore struct {
	blobs      map[string][]byte
	failOn     string
	readFails  bool
	deleteFail bool
}

func newStore() *memoryStore {
	return &memoryStore{blobs: map[string][]byte{}}
}

func (store *memoryStore) SignUpload(key blobs.Key, upload blobs.UploadTerms, expiresAt time.Time) (string, error) {
	if store.failOn == "upload" {
		return "", errors.New("sign failed")
	}
	return "https://blobs.test/" + key.Path() + "?put&type=" + upload.MediaType, nil
}

func (store *memoryStore) SignDownload(key blobs.Key, download blobs.DownloadTerms, expiresAt time.Time) (string, error) {
	if store.failOn == "download" {
		return "", errors.New("sign failed")
	}
	return "https://blobs.test/" + key.Path() + "?get&name=" + download.FileName, nil
}

func (store *memoryStore) Read(key blobs.Key) (io.ReadCloser, error) {
	if store.readFails {
		return nil, errors.New("read failed")
	}
	content, found := store.blobs[key.Path()]
	if !found {
		return nil, blobs.NotFound{Path: key.Path()}
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

func (store *memoryStore) Write(key blobs.Key, body io.Reader, size int64) error {
	content, _ := io.ReadAll(body)
	store.blobs[key.Path()] = content
	return nil
}

func (store *memoryStore) Copy(from blobs.Key, to blobs.Key) error {
	store.blobs[to.Path()] = store.blobs[from.Path()]
	return nil
}

func (store *memoryStore) Delete(key blobs.Key) error {
	if store.deleteFail {
		return errors.New("delete failed")
	}
	delete(store.blobs, key.Path())
	return nil
}

func (store *memoryStore) put(uploadUrl string, content string) {
	path := strings.TrimPrefix(strings.Split(uploadUrl, "?")[0], "https://blobs.test/")
	store.blobs[path] = []byte(content)
}

type tokenGate struct{}

func (tokenGate) Identify(authorization string) (oidc.Caller, error) {
	parts := strings.Split(strings.TrimPrefix(authorization, "Bearer "), "@")
	if len(parts) != 2 {
		return oidc.Caller{}, errors.New("bad token")
	}
	roles := []string{"Member"}
	if parts[0] == "directory" {
		roles = []string{"Service"}
	}
	return oidc.Caller{MemberId: parts[0], DomainId: parts[1], Roles: roles}, nil
}

type fixture struct {
	documents *memoryDocuments
	store     *memoryStore
	now       time.Time
	desk      Desk
}

func newFixture(t *testing.T) *fixture {
	fixture := &fixture{documents: newMemory(), store: newStore(), now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	shelf := Shelf{Documents: fixture.documents, Scroll: fixture.documents.Scroll, Known: collections.KnownCollections{Folder: t.TempDir()}}
	settings := Settings{}
	settings.Link.Seconds = 1800
	settings.Limit.Bytes = 1000
	fixture.desk = Desk{Gate: tokenGate{}, Shelf: shelf, Store: fixture.store, Settings: settings, Now: func() time.Time { return fixture.now }}
	return fixture
}

func digest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func encoded(text string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(text))
}

func recoverySlot() KeySlot {
	return KeySlot{Kind: "recovery", Salt: encoded("salt-salt-salt-1"), Rounds: 600000, WrappedKey: encoded("wrapped key bytes")}
}

func startFor(version int64, content string, device string) StartRequest {
	return StartRequest{ExpectedVersion: version, Size: int64(len(content)), Sha256: digest(content), KeySlots: []KeySlot{recoverySlot()}, DeviceId: device}
}

func (fixture *fixture) backedUp(t *testing.T, token string, version int64, content string) Backup {
	t.Helper()
	upload, err := fixture.desk.Start(token, startFor(version, content, "phone"))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	fixture.store.put(upload.UploadUrl, content)
	backup, err := fixture.desk.Complete(token, upload.Version)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	return backup
}
