package mailout

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/compose"
	"mesail.dev/packages/dkim"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/smtpclient"
)

func TestSettingsReaderFillsDefaults(t *testing.T) {
	settings := SettingsReader{Environment: func(string) string { return "" }}.Read()
	equal(t, 7200, settings.MaxAgeMinutes)
	equal(t, 5*24*time.Hour, settings.MaxAge())
	equal(t, time.Minute, settings.FirstDelay())
	equal(t, time.Hour, settings.MaxDelay())
	equal(t, 5*time.Minute, settings.Lease())
	equal(t, 30*time.Second, settings.Timeout())
	equal(t, "mail-out.localhost", settings.HeloName)
}

func TestSettingsReaderReadsTheYonSnapshot(t *testing.T) {
	environment := func(string) string {
		return `{"yon":{"mailout":{"maxageminutes":15,"relayaddress":"relay.example.net:587"}}}`
	}
	settings := SettingsReader{Environment: environment}.Read()
	equal(t, 15, settings.MaxAgeMinutes)
	equal(t, "relay.example.net:587", settings.RelayAddress)
}

func TestMissingBlobsRefuseEverything(t *testing.T) {
	store := MissingBlobs{}
	key := blobs.Key{DomainId: "example.com", Name: "outbound-1"}
	_, upload := store.SignUpload(key, blobs.UploadTerms{}, time.Now())
	_, download := store.SignDownload(key, blobs.DownloadTerms{}, time.Now())
	_, read := store.Read(key)
	for _, err := range []error{upload, download, read, store.Write(key, strings.NewReader(""), 0), store.Copy(key, key), store.Delete(key)} {
		equal(t, "mail-out has no blob folder", err.Error())
	}
}

func TestDeskFactoryChoosesStoresFromSettings(t *testing.T) {
	factory := DeskFactory{}
	equal(t, MissingBlobs{}, factory.blobs(Settings{}))
	equal(t, dkim.EnvironmentSecretSource{Prefix: "MESAIL_SECRET_"}, factory.secrets(Settings{SecretPrefix: "MESAIL_SECRET_"}))
	equal(t, dkim.FolderSecretSource{Root: "/keys"}, factory.secrets(Settings{SecretFolder: "/keys"}))
	direct, isDirect := factory.direct(Settings{DirectPort: 2525}).(smtpclient.DirectDelivery)
	equal(t, true, isDirect)
	equal(t, 2525, direct.Port)
}

func TestDeskFactoryReportsBrokenOutboxSettings(t *testing.T) {
	desk := DeskFactory{Environment: func(string) string { return `{"yon":{}}` }}.Desk(newMemoryDocuments())
	equal(t, true, desk.Failure != nil)
}

func TestOutcomeRules(t *testing.T) {
	rules := OutcomeRules{}
	equal(t, "deferred", rules.FromSmtp("relay", smtpclient.Reply{}, errors.New("dial failed")).Status)
	equal(t, "bounced", rules.FromSmtp("relay", smtpclient.Reply{}, smtpclient.DeliveryError{Reply: smtpclient.Reply{Code: 550, Enhanced: "5.7.1"}, Permanent: true}).Status)
	equal(t, "sent", rules.FromSmtp("relay", smtpclient.Reply{Code: 250, Text: "ok"}, nil).Status)
	equal(t, "mail-in unreachable: down", rules.FromHandOff(0, errors.New("down")).Detail)
	equal(t, "mail-in answered 503", rules.FromHandOff(503, nil).Detail)
	equal(t, "failed", rules.FromBuild(compose.MissingField{Field: "From"}).Status)
	equal(t, "deferred", rules.FromBuild(errors.New("no key")).Status)
	equal(t, Outcome{Status: "deferred", Transport: "queue", Detail: "store down"}, rules.Unavailable(errors.New("store down")))
	equal(t, true, rules.MailboxGone("5.1.10"))
	equal(t, false, rules.MailboxGone("5.7.1"))
}

func TestBackoffDoublesUpToTheCap(t *testing.T) {
	backoff := Backoff{First: time.Minute, Largest: time.Hour}
	equal(t, time.Minute, backoff.Delay(1))
	equal(t, 4*time.Minute, backoff.Delay(3))
	equal(t, time.Hour, backoff.Delay(40))
}

func TestMessageIdsKeepAQualifiedId(t *testing.T) {
	equal(t, "abc@example.org", MessageIds{}.Header(Item{MessageId: "<abc@example.org>", DomainId: "example.com"}))
	equal(t, "abc@example.com", MessageIds{}.Header(Item{MessageId: "abc", DomainId: "example.com"}))
}

func TestAddressesIgnoreBadText(t *testing.T) {
	equal(t, MailboxAddress{}, Addresses{}.Mailbox("not an address"))
	equal(t, "example.org", Addresses{}.Domain("Eve@Example.ORG"))
}

func TestStatesMapExpiredToFailedEvents(t *testing.T) {
	equal(t, "failed", States{}.EventStatus("expired"))
	equal(t, "sent", States{}.EventStatus("sent"))
}

func TestDocumentStoreReadsEveryPage(t *testing.T) {
	documents := newMemoryDocuments()
	store := DocumentStore{Documents: documents}
	collection, err := store.Ready("outbound-queue", "example.com")
	equal(t, nil, err)
	for range 150 {
		_, _ = documents.Insert(collection, Item{State: "queued"})
	}
	all, err := store.All(collection, map[string]any{"state": "queued"})
	equal(t, nil, err)
	equal(t, 150, len(all))
}

func TestDocumentStoreReportsBadAnswers(t *testing.T) {
	_, _, notJson := DocumentStore{}.window(make(chan int))
	_, _, notWindow := DocumentStore{}.window("text")
	_, _, notList := DocumentStore{}.window(map[string]any{"content": "text"})
	equal(t, true, notJson != nil)
	equal(t, true, notWindow != nil)
	equal(t, true, notList != nil)
}

func TestStoresReportStorageFailures(t *testing.T) {
	for _, method := range []string{"CreateCollection", "Insert", "FindById", "Patch", "Command"} {
		t.Run(method, func(t *testing.T) {
			documents := newMemoryDocuments()
			store := DocumentStore{Documents: documents}
			registry := Registry{Store: store}
			queue := Queue{Store: store, Registry: registry, Outbox: outbox.Settings{Producer: "mail-out"}}
			item := Item{DomainId: "example.com", State: "queued"}
			change := events.Envelope{EventID: "e", Type: events.Types{}.EmailDeliveryChanged(), DomainId: "example.com", Producer: "mail-out", OccurredAt: Clock{}.Format(time.Now()), Payload: []byte(`{}`)}
			saved, _ := queue.Insert(item, change)
			documents.fail(method, errors.New("storage down"))
			_, insertErr := queue.Insert(item, change)
			_, _, claimErr := queue.Claim(saved, time.Now())
			_, _, findErr := queue.FindBy("example.com", "eventId", "e")
			_, dueErr := queue.Due("example.com", time.Now())
			_, collectionsErr := registry.Collections("outbound-queue")
			containsErr := error(nil)
			_, containsErr = Suppressions{Store: store}.Contains("example.com", "a@b.c")
			addErr := Suppressions{Store: store}.Add(Suppression{DomainId: "example.com", Address: "a@b.c"})
			recordErr := AttemptLog{Store: store}.Record(DeliveryAttempt{DomainId: "example.com"})
			_, forItemErr := AttemptLog{Store: store}.ForItem("example.com", "x")
			_, seenErr := SeenEvents{Store: store, Now: time.Now}.Seen("example.com", "e")
			rememberErr := SeenEvents{Store: store, Now: time.Now}.Remember("example.com", "e")
			_, pendingErr := OutboxStore{Store: store, Registry: registry}.Pending(10)
			failures := []error{insertErr, claimErr, findErr, dueErr, collectionsErr, containsErr, addErr, recordErr, forItemErr, seenErr, rememberErr, pendingErr, queue.Save(saved), queue.SaveWith(saved, change)}
			count := 0
			for _, err := range failures {
				if err != nil {
					count++
				}
			}
			equal(t, true, count > 0)
		})
	}
}

func TestQueueSaveKeepsTheStoredOutboxFields(t *testing.T) {
	documents := newMemoryDocuments()
	store := DocumentStore{Documents: documents}
	queue := Queue{Store: store, Registry: Registry{Store: store}, Outbox: outbox.Settings{Producer: "mail-out", Subscribers: []outbox.Subscriber{{Name: "messaging", URL: "http://127.0.0.1:1"}}}}
	change := events.Envelope{EventID: "e", Type: events.Types{}.EmailDeliveryChanged(), DomainId: "example.com", Producer: "mail-out", OccurredAt: Clock{}.Format(time.Now()), Payload: []byte(`{}`)}
	saved, err := queue.Insert(Item{DomainId: "example.com", State: "queued"}, change)
	equal(t, nil, err)
	changed := saved
	changed.Detail, changed.PendingEvents, changed.OutboxPending = "checked", nil, 0

	equal(t, nil, queue.Save(changed))
	after, _, _ := queue.FindBy("example.com", "detail", "checked")
	equal(t, 1, after.OutboxPending)
	equal(t, saved.PendingEvents, after.PendingEvents)
}

func TestAnUnstageableChangeStillSavesTheState(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.mustSend(envelope)
	item := w.item("example.com", envelope.EventId)
	outcomes := w.desk.Run.Delivery.Outcomes
	outcomes.Changes = ChangeEvents{Now: time.Now}
	item.DomainId = "Not A Domain"

	_, err := outcomes.Apply(item, Outcome{Status: "sent"}, time.Now())
	equal(t, true, err != nil)
}

func TestServiceClientReportsProblems(t *testing.T) {
	broken := ServiceClient{Client: http.DefaultClient, Authorize: func(string) (string, error) { return "", errors.New("no token") }}
	working := ServiceClient{Client: http.DefaultClient, Authorize: fixedTokens}
	_, tokenErr := broken.Get("http://127.0.0.1:1", "example.com", nil)
	_, badAddress := working.Get("http://[::1", "example.com", nil)
	_, postAddress := working.Post("http://[::1", "example.com", nil)
	_, encodeErr := working.Post("http://127.0.0.1:1", "example.com", make(chan int))
	_, unreachable := working.Post("http://127.0.0.1:1", "example.com", nil)
	for _, err := range []error{tokenErr, badAddress, postAddress, encodeErr, unreachable} {
		equal(t, true, err != nil)
	}
}

func TestDirectoryReportsServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	_, found, err := Directory{Client: ServiceClient{Client: http.DefaultClient, Authorize: fixedTokens}, BaseUrl: server.URL}.Domain("example.com", "example.com")
	equal(t, false, found)
	equal(t, "directory answered 500", err.Error())
}

func TestRelayerReportsTokenProblems(t *testing.T) {
	relayer := Relayer{Authorize: func(string) (string, error) { return "", errors.New("no token") }}
	_, err := relayer.RelayDocument(outbox.Ref{Collection: "c", Id: "i"}, "example.com")
	equal(t, "no token", err.Error())
}

func TestIntakeReportsStorageFailures(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.documents.fail("Insert", errors.New("storage down"))
	_, insertErr := w.send(envelope)
	w.documents.fail("Command", errors.New("storage down"))
	_, findErr := w.desk.Receive("Bearer token-example.com", w.event("example.com", "alice@example.com", "bob@partner.test"))
	equal(t, true, insertErr != nil)
	equal(t, true, findErr != nil)
}

func TestARunReportsStorageFailures(t *testing.T) {
	w := newWorld(t, nil)
	w.mustSend(w.event("example.com", "alice@example.com", "bob@partner.test"))
	w.documents.fail("Patch", errors.New("storage down"))
	_, claimErr := w.desk.Run.Run(time.Now())
	w.documents.fail("Command", errors.New("storage down"))
	_, dueErr := w.desk.Run.Run(time.Now())
	_, relayErr := w.desk.Relay()
	_, deadErr := w.desk.DeadLetters()
	equal(t, true, claimErr != nil)
	equal(t, true, dueErr != nil)
	equal(t, true, relayErr != nil)
	equal(t, true, deadErr != nil)
}

func TestDeliveryDefersWhenTheSuppressionListFails(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.mustSend(envelope)
	item := w.item("example.com", envelope.EventId)
	delivery := w.desk.Run.Delivery
	delivery.Suppressions = Suppressions{Store: DocumentStore{Documents: failingCollections{w.documents}}}

	_, outcome := delivery.attempt(item, time.Now())
	equal(t, "deferred", outcome.Status)
}

type failingCollections struct {
	*memoryDocuments
}

func (failingCollections) CreateCollection(string) error { return errors.New("storage down") }
