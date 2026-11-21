package devicebox

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

type brokenStorage struct {
	answer any
}

func (storage brokenStorage) Command(command map[string]any) (any, error) {
	if storage.answer != nil {
		return storage.answer, nil
	}
	return nil, errors.New("disk on fire")
}

func (brokenStorage) Query(sql string, parameters []any) (any, error) {
	return nil, errors.New("disk on fire")
}

func TestStorageFailuresAreUnavailableAndKeepTheirCause(t *testing.T) {
	documents := NewDocuments(brokenStorage{})
	err := documents.Ensure("slips")
	unavailable := &Unavailable{}
	if !errors.As(err, &unavailable) || unavailable.Number() != 6021 || unavailable.Unwrap() == nil || unavailable.Error() == "" {
		t.Fatalf("got %v", err)
	}
	if _, err := documents.Insert("slips", map[string]any{"a": 1}); numberOf(err) != 6021 {
		t.Fatalf("got %v", err)
	}
	if found, err := documents.Find("slips", "X", &Slip{}); found || numberOf(err) != 6021 {
		t.Fatalf("got %v", err)
	}
	if err := documents.Patch("slips", "X", map[string]any{"a": 1}); numberOf(err) != 6021 {
		t.Fatalf("got %v", err)
	}
	if err := documents.Rows(Statement{Sql: "SELECT"}, &[]Slip{}); numberOf(err) != 6021 {
		t.Fatalf("got %v", err)
	}
	if err := NewDocuments(brokenStorage{answer: "text"}).into(map[string]any{}, &[]Slip{}); numberOf(err) != 6022 {
		t.Fatalf("got %v", err)
	}
	if err := documents.decode(make(chan int), &Slip{}); numberOf(err) != 6022 {
		t.Fatalf("got %v", err)
	}
}

func TestStoreAndCountsPassStorageFailuresOn(t *testing.T) {
	store := NewStore(NewDocuments(brokenStorage{}))
	if err := store.EnsureDomain("example.com"); err == nil {
		t.Fatal("want a failure")
	}
	if _, _, err := store.SlipByEnvelope("example.com", "x"); err == nil {
		t.Fatal("want a failure")
	}
	if _, err := store.Waiting("example.com", DeviceKey{}.Of("b1"), 0, 1); err == nil {
		t.Fatal("want a failure")
	}
	if _, err := store.Domains(); err == nil {
		t.Fatal("want a failure")
	}
	if _, err := NewAcknowledgementRun(store).Remove("example.com", "k", []string{"x"}); err == nil {
		t.Fatal("want a failure")
	}
	if err := NewBoxCounts(NewDocuments(brokenStorage{})).Room("example.com", []string{DeviceKey{}.Of("b1")}); err == nil {
		t.Fatal("want a failure")
	}
	if _, err := NewBoxCounts(NewDocuments(brokenStorage{})).Add("example.com", map[string]int{DeviceKey{}.Of("b1"): 1}); err == nil {
		t.Fatal("want a failure")
	}
	if _, err := NewOutboxStore(NewDocuments(brokenStorage{})).Waiting(); err == nil {
		t.Fatal("want a failure")
	}
	if _, err := NewOutboxStore(NewDocuments(brokenStorage{})).Load(outbox.Ref{Collection: "c", Id: "X"}); err == nil {
		t.Fatal("want a failure")
	}
	expiry := Expiry{callers: fakeCallers{}, parts: ExpiryParts{Store: store}, now: time.Now}
	if _, err := expiry.Run("* job Service"); err == nil {
		t.Fatal("want a failure")
	}
}

func TestMissingOutboxSettingsMakeStagingAndTheRelayUnavailable(t *testing.T) {
	settings := SettingsSource{}.FromSnapshot(`{}`)
	_, err := NewEventStaging(settings, time.Now).Staged("example.com", Delivery{DeviceIds: []string{"b1"}, Owners: map[string]string{"b1": "bo"}})
	if numberOf(err) != 6023 {
		t.Fatalf("got %v", err)
	}
	desk := NewOutboxDesk(NewDocuments(brokenStorage{}), outbox.NewRecordingPusher(), settings, time.Now)
	if _, err := desk.RunOnce(); numberOf(err) != 6023 {
		t.Fatalf("got %v", err)
	}
	desk.Drain("c", "X")
}

func TestAFullOutboxIsUnavailableAndOtherFailuresAreUnreadable(t *testing.T) {
	if numberOf(EventStaging{}.outboxError(outbox.OutboxFull{})) != 17 {
		t.Fatal("want the kit number 17")
	}
	if numberOf(EventStaging{}.outboxError(errors.New("bad"))) != 6030 {
		t.Fatal("want 6030")
	}
	if numberOf(OutboxDesk{}.letterError(outbox.NotFound{}.DeadLetter("evt_1 for realtime"))) != 15 || numberOf(OutboxDesk{}.letterError(errors.New("bad"))) != 6030 {
		t.Fatal("letter errors")
	}
	if (ExpiryRun{}).firstError(nil, errors.New("second")).Error() != "second" {
		t.Fatal("firstError")
	}
}

func TestTheRelayDeliversEventsThatWaitedAndRequeuesDeadLetters(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	w.pusher.FailFor("realtime", outbox.DeliveryFailure{Subscriber: "realtime", Status: http.StatusBadRequest, Permanent: true})
	w.mustPut("ana", "a1", "b1")
	dead, err := w.desk.DeadEvents()
	if err != nil || len(dead) != 1 || dead[0].Subscriber != "realtime" || dead[0].Type != "envelope.stored" {
		t.Fatalf("got %+v %v", dead, err)
	}
	w.pusher.Recover("realtime")
	if err := w.desk.RetryDead(outbox.Ref{Collection: dead[0].Collection, Id: dead[0].DocumentId}, dead[0].EventId, "realtime"); err != nil {
		t.Fatal(err)
	}
	report, err := w.desk.RunOnce()
	if err != nil || report.Delivered != 1 {
		t.Fatalf("got %+v %v", report, err)
	}
	if err := w.desk.RetryDead(outbox.Ref{Collection: dead[0].Collection, Id: dead[0].DocumentId}, "evt_none", "realtime"); numberOf(err) != 15 {
		t.Fatalf("got %v", err)
	}
	if err := w.desk.RetryDead(outbox.Ref{Collection: "elsewhere", Id: "X"}, "evt_none", "realtime"); numberOf(err) != 6027 {
		t.Fatalf("got %v", err)
	}
	if err := w.desk.RetryDead(outbox.Ref{Collection: dead[0].Collection, Id: "4WPKAAAAAAA"}, "evt_none", "realtime"); numberOf(err) != 6026 {
		t.Fatalf("got %v", err)
	}
}

func TestTheScheduledCallPostsWithAnInstallWideToken(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	asked := oidc.ServiceTokenRequest{}
	identity := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		json.NewDecoder(request.Body).Decode(&asked)
		writer.Write([]byte(`{"accessToken":"example-token","expiresIn":60}`))
	}))
	defer identity.Close()
	seen := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen = append(seen, request.Method+" "+request.URL.Path+" "+request.Header.Get("Authorization"))
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	port := server.URL[strings.LastIndex(server.URL, ":")+1:]
	settings := fmt.Sprintf(`{"yon":{"server":{"port":%s},"oidc":{"servicetokens":%q},"service":{"client":{"secret":"example-secret"}}}}`, port, identity.URL)
	err := ScheduledCall{Path: "/api/v1/devicebox/jobs/expire"}.Run(settings)
	if err != nil || len(seen) != 1 || seen[0] != "POST /api/v1/devicebox/jobs/expire Bearer example-token" || asked.DomainId != "*" {
		t.Fatalf("got %v %v %+v", seen, err, asked)
	}
	noPort := fmt.Sprintf(`{"yon":{"oidc":{"servicetokens":%q},"service":{"client":{"secret":"example-secret"}}}}`, identity.URL)
	if err := (ScheduledCall{}).Run(noPort); err == nil {
		t.Fatal("a missing port should fail")
	}
	if err := (ScheduledCall{}).Run(`{"yon":{}}`); err == nil {
		t.Fatal("missing token settings should fail")
	}
	if (ScheduledCall{}).outcome(http.StatusTooManyRequests) == nil || (ScheduledCall{}).outcome(http.StatusOK) != nil {
		t.Fatal("outcome")
	}
}

func TestMemoryAndUnwiredDirectories(t *testing.T) {
	directory := NewMemoryDirectory()
	directory.Owns("example.com", "bo", "b1")
	owners, err := directory.Owners("example.com", []string{"b1", "b2"})
	if err != nil || len(owners) != 1 || owners["b1"] != "bo" {
		t.Fatalf("got %v %v", owners, err)
	}
	if _, err := (UnwiredDirectory{}).Owners("example.com", nil); numberOf(err) != 6020 {
		t.Fatalf("got %v", err)
	}
}

func TestTheAssemblyWiresTheDirectoryFromItsSettings(t *testing.T) {
	if _, unwired := (Assembly{}).directoryAt("").(UnwiredDirectory); !unwired {
		t.Fatal("no address should mean the unwired directory")
	}
	if _, client := (Assembly{}).directoryAt("http://127.0.0.1:8102").(DirectoryClient); !client {
		t.Fatal("an address should mean the HTTP client")
	}
	settings := SettingsSource{}.FromSnapshot(`{"yon":{"devicebox":{"directoryurl":"http://127.0.0.1:8102"},"outbox":{"producer":"devicebox","subscribers":[]}}}`)
	if settings.DirectoryUrl != "http://127.0.0.1:8102" || settings.Checked() != nil {
		t.Fatalf("got %+v", settings)
	}
}

func TestTheRelaySweepHasTwoSecondsAndTheExpiryJobKeepsTwenty(t *testing.T) {
	if (Limits{}).SweepBudget() != 2*time.Second || (Limits{}).TimeBudget() != 20*time.Second {
		t.Fatalf("sweep %s, expiry %s", Limits{}.SweepBudget(), Limits{}.TimeBudget())
	}
}
