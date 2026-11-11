package attachments

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
)

type memoryDocuments struct {
	collections map[string]map[string][]byte
	failOn      string
	created     int
}

func newMemory() *memoryDocuments {
	return &memoryDocuments{collections: map[string]map[string][]byte{}}
}

func (memory *memoryDocuments) failure(operation string) error {
	if memory.failOn == operation {
		return errors.New(operation + " failed")
	}
	return nil
}

func (memory *memoryDocuments) CollectionExists(name string) (bool, error) {
	_, found := memory.collections[name]
	return found, memory.failure("exists")
}

func (memory *memoryDocuments) CreateCollection(name string) error {
	if err := memory.failure("create"); err != nil {
		return err
	}
	memory.created++
	memory.collections[name] = map[string][]byte{}
	return nil
}

func (memory *memoryDocuments) Insert(collection string, document any) (string, error) {
	id := "D" + strconv.Itoa(len(memory.collections[collection])+1)
	raw, _ := json.Marshal(document)
	memory.collections[collection][id] = raw
	return id, memory.failure("insert")
}

func (memory *memoryDocuments) FindById(collection string, id string, target any) (bool, error) {
	raw, found := memory.collections[collection][id]
	json.Unmarshal(raw, target)
	return found, memory.failure("find")
}

func (memory *memoryDocuments) Patch(collection string, id string, fields any, target any) (bool, error) {
	current := map[string]any{}
	json.Unmarshal(memory.collections[collection][id], &current)
	changes, _ := json.Marshal(fields)
	json.Unmarshal(changes, &current)
	memory.collections[collection][id], _ = json.Marshal(current)
	return true, memory.failure("patch")
}

type fakeGate struct{}

func (fakeGate) Identify(authorization string) (oidc.Caller, error) {
	parts := strings.Split(strings.TrimPrefix(authorization, "Bearer "), "|")
	if len(parts) != 3 {
		return oidc.Caller{}, errors.New("refused")
	}
	return oidc.Caller{DomainId: parts[0], MemberId: parts[1], Roles: []string{parts[2]}}, nil
}

type fakeReaders struct {
	allowed bool
	asked   int
}

func (readers *fakeReaders) MayRead(attachmentId string, authorization string) bool {
	readers.asked++
	return readers.allowed
}

func token(domainId string, memberId string) string {
	return "Bearer " + domainId + "|" + memberId + "|Member"
}

func fixedNow() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}

type testDesk struct {
	desk    Desk
	memory  *memoryDocuments
	folder  blobs.LocalFolder
	readers *fakeReaders
}

func newTestDesk(t *testing.T) testDesk {
	folder := blobs.NewLocalFolder(t.TempDir(), "http://127.0.0.1:8100/blobs", blobs.NewLocalSigner([]byte("secret")))
	memory := newMemory()
	readers := &fakeReaders{}
	settings := Settings{Link: LinkSettings{Seconds: 300}, Limit: LimitSettings{Bytes: 100}}
	desk := Desk{Gate: fakeGate{}, Book: RecordBook{Documents: memory, Now: fixedNow}, Store: folder, Access: AccessRules{Readers: readers}, Settings: settings, Now: fixedNow}
	return testDesk{desk: desk, memory: memory, folder: folder, readers: readers}
}

func request(content string, mediaType string) UploadRequest {
	inspection, _ := blobs.Inspector{}.Inspect(strings.NewReader(content), int64(len(content)))
	return UploadRequest{FileName: "plan.txt", MediaType: mediaType, Size: inspection.Size, Sha256: inspection.Sha256}
}

func (desk testDesk) uploaded(t *testing.T, authorization string, content string, stored string) Upload {
	upload, err := desk.desk.Create(authorization, request(content, "text/plain"))
	if err != nil {
		t.Fatal(err)
	}
	record, _ := desk.desk.Book.Lookup(strings.Split(strings.TrimPrefix(authorization, "Bearer "), "|")[0], upload.AttachmentId)
	desk.folder.Write(desk.desk.Book.Key(record), strings.NewReader(stored), int64(len(stored)))
	return upload
}

type brokenStore struct {
	blobs.LocalFolder
}

func (brokenStore) Read(key blobs.Key) (io.ReadCloser, error) {
	return io.NopCloser(brokenReader{}), nil
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) {
	return 0, errors.New("disk gone")
}
