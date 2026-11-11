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
	for _, operation := range []string{"exists", "create", "find"} {
		memory := newMemory()
		memory.failOn = operation
		book := RecordBook{Documents: memory, Now: fixedNow}
		_, lookupErr := book.Lookup("example.com", "att_D1")
		_, stateErr := book.SetState(Record{DomainId: "example.com", ID: "D1"}, "ready")
		var unavailable Unavailable
		if !errors.As(lookupErr, &unavailable) || (operation != "find" && !errors.As(stateErr, &unavailable)) {
			t.Fatalf("%s: got %v and %v", operation, lookupErr, stateErr)
		}
	}
}

func TestRecordBookCreatesEachCollectionOnce(t *testing.T) {
	memory := newMemory()
	book := RecordBook{Documents: memory, Now: fixedNow}
	first, _ := book.Collection("example.com")
	second, _ := book.Collection("example.com")
	other, _ := book.Collection("example.org")
	if first != second || first == other || memory.created != 2 || !strings.HasPrefix(first, "attachments-") {
		t.Fatalf("got %s, %s, %s after %d creations", first, second, other, memory.created)
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
	want := Settings{Adapter: "local", Bucket: "mesail-attachments", Region: "us-east-1", Link: LinkSettings{Seconds: 300}, Limit: LimitSettings{Bytes: 26214400}}
	if settings != want {
		t.Fatalf("got %+v", settings)
	}
}

func TestSettingsReaderReadsTheSnapshotAndTheAwsEnvironment(t *testing.T) {
	snapshot := `{"yon":{"attachments":{"adapter":"s3","bucket":"files","messaging":"http://messaging:8104","link":{"seconds":60,"secret":"s"},"limit":{"bytes":5}}}}`
	settings := SettingsReader{Environment: environment(map[string]string{"YON_APPLICATION_JSON": snapshot, "AWS_REGION": "eu-west-1", "AWS_ENDPOINT_URL": "http://floci:4566"})}.Read()
	if settings.Adapter != "s3" || settings.Bucket != "files" || settings.Region != "eu-west-1" || settings.Endpoint != "http://floci:4566" || settings.Link.Seconds != 60 || settings.Limit.Bytes != 5 || settings.Messaging != "http://messaging:8104" {
		t.Fatalf("got %+v", settings)
	}
}

func TestStoreFactoryBuildsEachAdapter(t *testing.T) {
	factory := StoreFactory{Environment: environment(map[string]string{"AWS_ACCESS_KEY_ID": "test"})}
	local, localErr := factory.Store(Settings{Adapter: "local", Folder: t.TempDir(), Link: LinkSettings{Base: "http://127.0.0.1:8100/blobs", Secret: "s"}})
	s3, s3Err := factory.Store(Settings{Adapter: "s3", Bucket: "files", Region: "us-east-1", Endpoint: "http://floci:4566", Public: "http://127.0.0.1:14566"})
	key := blobs.Key{DomainId: "example.com", Name: "B1"}
	localUrl, _ := local.SignUpload(key, blobs.UploadTerms{Size: 1}, fixedNow())
	s3Url, _ := s3.SignUpload(key, blobs.UploadTerms{Size: 1}, fixedNow().Add(time.Minute))
	if localErr != nil || s3Err != nil || !strings.HasPrefix(localUrl, "http://127.0.0.1:8100/blobs/example.com/B1?") || !strings.HasPrefix(s3Url, "http://127.0.0.1:14566/files/example.com/B1?") {
		t.Fatalf("got %s and %s (%v, %v)", localUrl, s3Url, localErr, s3Err)
	}
}

func TestStoreFactoryRefusesLocalLinksWithoutASecret(t *testing.T) {
	var unavailable Unavailable
	if _, err := (StoreFactory{Environment: environment(nil)}).Store(Settings{Adapter: "local"}); !errors.As(err, &unavailable) {
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
	messages := []string{Invalid{Field: "size", Reason: "too small"}.Error(), Conflict{Field: "state", Reason: "not ready"}.Error(), Missing{}.Error(), Unavailable{Operation: "read"}.Error(), Unauthenticated{}.Error()}
	wanted := []string{"size: too small", "state: not ready", "the attachment was not found", "attachment storage failed during read", "the access token was refused"}
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
