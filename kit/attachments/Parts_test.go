package attachments

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
)

func TestUploadRulesAcceptAValidRequest(t *testing.T) {
	if err := (UploadRules{}).Check(request("hello", "text/plain"), 5); err != nil {
		t.Fatal(err)
	}
}

func TestUploadRulesNameTheBadField(t *testing.T) {
	cases := map[string]func(*UploadRequest){
		"fileName":  func(r *UploadRequest) { r.FileName = "../plan.txt" },
		"mediaType": func(r *UploadRequest) { r.MediaType = "plain" },
		"size":      func(r *UploadRequest) { r.Size = 0 },
		"sha256":    func(r *UploadRequest) { r.Sha256 = strings.Repeat("A", 64) },
	}
	for field, change := range cases {
		changed := request("hello", "text/plain")
		change(&changed)
		var invalid Invalid
		if err := (UploadRules{}).Check(changed, 100); !errors.As(err, &invalid) || invalid.Field != field {
			t.Fatalf("%s: got %v", field, err)
		}
	}
}

func TestUploadRulesRefuseControlCharactersAndLongNames(t *testing.T) {
	for _, name := range []string{"", "a\nb", strings.Repeat("n", 256), "a\\b", string([]byte{0xff})} {
		changed := request("hello", "text/plain")
		changed.FileName = name
		if err := (UploadRules{}).Check(changed, 100); err == nil {
			t.Fatalf("%q accepted", name)
		}
	}
}

func TestRecordBookReportsStorageFailures(t *testing.T) {
	cases := []struct {
		operation string
		failing   func(book RecordBook) error
	}{
		{"exists", func(book RecordBook) error { _, err := book.Insert(Record{DomainId: "example.com"}); return err }},
		{"exists", func(book RecordBook) error { _, err := book.Lookup("example.com", "att_D1"); return err }},
		{"create", func(book RecordBook) error { _, err := book.Insert(Record{DomainId: "example.com"}); return err }},
		{"insert", func(book RecordBook) error { _, err := book.Insert(Record{DomainId: "example.com"}); return err }},
		{"find", func(book RecordBook) error { _, err := book.Lookup("example.org", "att_D1"); return err }},
		{"patch", func(book RecordBook) error {
			_, err := book.SetState(Record{DomainId: "example.org", ID: "D1"}, "ready")
			return err
		}},
	}
	for _, test := range cases {
		memory := newMemory()
		book := RecordBook{Documents: memory, Now: fixedNow}
		book.Insert(Record{DomainId: "example.org"})
		memory.failOn = test.operation
		var unavailable Unavailable
		if err := test.failing(book); !errors.As(err, &unavailable) {
			t.Fatalf("%s: got %v", test.operation, err)
		}
	}
}

func TestRecordBookCreatesEachCollectionOnce(t *testing.T) {
	memory := newMemory()
	book := RecordBook{Documents: memory, Now: fixedNow}
	book.Insert(Record{DomainId: "example.com", FileName: "a.txt"})
	book.Insert(Record{DomainId: "example.com", FileName: "b.txt"})
	book.Insert(Record{DomainId: "example.org", FileName: "c.txt"})
	first, other := book.Collection("example.com"), book.Collection("example.org")
	if first == other || memory.created != 2 || !strings.HasPrefix(first, "attachments-") {
		t.Fatalf("got %s, %s after %d creations", first, other, memory.created)
	}
}

func TestRecordBookLookupInANewDomainCreatesNothing(t *testing.T) {
	memory := newMemory()
	book := RecordBook{Documents: memory, Now: fixedNow}
	_, err := book.Lookup("example.com", "att_A1")
	if !errors.As(err, &Missing{}) || memory.created != 0 {
		t.Fatalf("got %v after %d creations", err, memory.created)
	}
}

func TestRecordBookRefusesBadIds(t *testing.T) {
	book := RecordBook{Documents: newMemory(), Now: fixedNow}
	for _, id := range []string{"msg_D1", "att_", "att_../x", "D1"} {
		if _, err := book.Lookup("example.com", id); !errors.Is(err, Missing{}) {
			t.Fatalf("%q: got %v", id, err)
		}
	}
}

func TestRecordBookInsertReportsACollectionFailure(t *testing.T) {
	memory := newMemory()
	memory.failOn = "exists"
	if _, err := (RecordBook{Documents: memory, Now: fixedNow}).Insert(Record{DomainId: "example.com"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestRecordBookKeyAndView(t *testing.T) {
	book := RecordBook{}
	record := Record{ID: "D1", FileName: "a.txt", MediaType: "text/plain", Size: 3, Sha256: "abc", State: "ready", BlobKey: "example.com/B1"}
	if key := book.Key(record); key != (blobs.Key{DomainId: "example.com", Name: "B1"}) {
		t.Fatalf("got %+v", key)
	}
	want := View{AttachmentId: "att_D1", FileName: "a.txt", MediaType: "text/plain", Size: 3, Sha256: "abc", State: "ready"}
	if view := book.View(record); view != want {
		t.Fatalf("got %+v", view)
	}
}

func TestMessagingAccessAsksWithTheCallersToken(t *testing.T) {
	seen := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen = request.URL.Path + " " + request.Header.Get("Authorization")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	allowed := MessagingAccess{BaseUrl: server.URL, Client: server.Client()}.MayRead("att_D1", "Bearer t")
	if !allowed || seen != "/api/v1/messaging/attachments/att_D1/access Bearer t" {
		t.Fatalf("got %v, %q", allowed, seen)
	}
}

func TestMessagingAccessFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	answers := []bool{
		MessagingAccess{BaseUrl: server.URL, Client: server.Client()}.MayRead("att_D1", "Bearer t"),
		MessagingAccess{BaseUrl: "http://127.0.0.1:1", Client: &http.Client{Timeout: time.Second}}.MayRead("att_D1", "Bearer t"),
		MessagingAccess{BaseUrl: "::bad", Client: http.DefaultClient}.MayRead("att_D1", "Bearer t"),
		MessagingAccess{BaseUrl: "", Client: http.DefaultClient}.MayRead("att_D1", "Bearer t"),
	}
	for index, allowed := range answers {
		if allowed {
			t.Fatalf("case %d allowed", index)
		}
	}
}

func TestAccessRulesTrustTheUploaderAndServices(t *testing.T) {
	readers := &fakeReaders{}
	rules := AccessRules{Readers: readers}
	record := Record{UploaderId: "ana"}
	uploader := rules.Check(oidc.Caller{MemberId: "ana"}, record, ReadQuestion{})
	service := rules.Check(oidc.Caller{MemberId: "mail-out", Roles: []string{"Service"}}, record, ReadQuestion{})
	other := rules.Check(oidc.Caller{MemberId: "bo", Roles: []string{"Member"}}, record, ReadQuestion{})
	if uploader != nil || service != nil || !errors.Is(other, Missing{}) || readers.asked != 1 {
		t.Fatalf("got %v, %v, %v after %d questions", uploader, service, other, readers.asked)
	}
}

func environment(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestSettingsReaderFallsBackToDefaults(t *testing.T) {
	settings := SettingsReader{Environment: environment(nil)}.Read()
	want := Settings{Adapter: "local", Link: LinkSettings{Seconds: 300}, Limit: LimitSettings{Bytes: 26214400}}
	if settings != want {
		t.Fatalf("got %+v", settings)
	}
}

func TestSettingsReaderReadsTheSnapshot(t *testing.T) {
	snapshot := `{"yon":{"attachments":{"adapter":"local","folder":"/data/blobs","messaging":"http://messaging:8104","link":{"seconds":60,"secret":"s"},"limit":{"bytes":5}}}}`
	settings := SettingsReader{Environment: environment(map[string]string{"YON_APPLICATION_JSON": snapshot})}.Read()
	if settings.Adapter != "local" || settings.Folder != "/data/blobs" || settings.Link.Seconds != 60 || settings.Limit.Bytes != 5 || settings.Messaging != "http://messaging:8104" {
		t.Fatalf("got %+v", settings)
	}
}

func TestStoreFactoryBuildsTheLocalAdapter(t *testing.T) {
	local, err := StoreFactory{}.Store(Settings{Adapter: "local", Folder: t.TempDir(), Link: LinkSettings{Base: "http://127.0.0.1:8100/blobs", Secret: "s"}})
	key := blobs.Key{DomainId: "example.com", Name: "B1"}
	localUrl, _ := local.SignUpload(key, blobs.UploadTerms{Size: 1}, fixedNow())
	if err != nil || !strings.HasPrefix(localUrl, "http://127.0.0.1:8100/blobs/example.com/B1?") {
		t.Fatalf("got %s (%v)", localUrl, err)
	}
}

func TestStoreFactoryRefusesTheRemovedS3Adapter(t *testing.T) {
	var removed RemovedSetting
	store, err := StoreFactory{}.Store(Settings{Adapter: "s3", Link: LinkSettings{Secret: "s"}})
	if store != nil || !errors.As(err, &removed) || removed.Name != "adapter" || !strings.Contains(err.Error(), `adapter "s3" was removed`) {
		t.Fatalf("got %v and %v", store, err)
	}
}

func TestStoreFactoryRefusesAnS3Bucket(t *testing.T) {
	var removed RemovedSetting
	store, err := StoreFactory{}.Store(Settings{Adapter: "local", Bucket: "mesail-attachments", Link: LinkSettings{Secret: "s"}})
	if store != nil || !errors.As(err, &removed) || removed.Name != "bucket" || !strings.Contains(err.Error(), "ADR 0007") {
		t.Fatalf("got %v and %v", store, err)
	}
}

func TestStoreFactoryRefusesLocalLinksWithoutASecret(t *testing.T) {
	var unavailable Unavailable
	if _, err := (StoreFactory{}).Store(Settings{Adapter: "local"}); !errors.As(err, &unavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestDeskFactoryBuildsAFailClosedDesk(t *testing.T) {
	desk := DeskFactory{Environment: environment(nil)}.Desk(newMemory())
	if desk.Failure == nil || desk.Settings.Adapter != "local" {
		t.Fatalf("got %+v", desk)
	}
	if _, err := desk.Create("Bearer anything", request("hello", "text/plain")); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestErrorsExplainThemselves(t *testing.T) {
	messages := []string{Invalid{Field: "size", Reason: "too small"}.Error(), Conflict{Field: "state", Reason: "not ready"}.Error(), Missing{}.Error(), Unavailable{Operation: "read"}.Error(), Unauthenticated{}.Error(), Forbidden{}.Error()}
	wanted := []string{"size: too small", "state: not ready", "the attachment was not found", "attachment storage failed during read", "the access token was refused", "only a service may read attachment details"}
	for index, message := range messages {
		if message != wanted[index] {
			t.Fatalf("got %q", message)
		}
	}
}

func TestStatesNameTheLifecycle(t *testing.T) {
	if (States{}).Pending() != "pending" || (States{}).Ready() != "ready" || (States{}).Blocked() != "blocked" {
		t.Fatal("unexpected state names")
	}
}

func TestVerifierReportsStoresThatFail(t *testing.T) {
	folder := blobs.NewLocalFolder(t.TempDir(), "http://127.0.0.1:8100/blobs", blobs.NewLocalSigner([]byte("s")))
	_, badKey := Verifier{}.Verify(folder, blobs.Key{DomainId: "example.com", Name: "../x"}, Record{Size: 1})
	_, broken := Verifier{}.Verify(brokenStore{folder}, blobs.Key{DomainId: "example.com", Name: "B1"}, Record{Size: 1})
	if !errors.Is(badKey, Unavailable{Operation: "read"}) || !errors.Is(broken, Unavailable{Operation: "inspect"}) {
		t.Fatalf("got %v and %v", badKey, broken)
	}
}
