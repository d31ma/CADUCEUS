import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"time"

	"mesail.dev/packages/collections"
	"mesail.dev/packages/events"
	"mesail.dev/packages/mailout"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @Test(MailOutTestSetup)
type OutboxDueTest struct {
	behaviors *BehaviorContext
	lock      sync.Mutex
	realm     oidc.SigningRealm
	pushed    []string
}

func (*OutboxDueTest) domain() string { return "due-order.test" }

func (*OutboxDueTest) subscriber() string { return "messaging" }

func (*OutboxDueTest) notDueCount() int { return 26 }

func (*OutboxDueTest) batchLimit() int { return 25 }

func (*OutboxDueTest) relayRounds() int { return 10 }

func (*OutboxDueTest) firstAttempts() int { return 1 }

func (test *OutboxDueTest) start() *httptest.Server {
	test.realm = MailOutTestSetup{}.Realm()
	test.pushed = nil
	test.dropQueue()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/messaging/events", test.messagingEvents)
	mux.HandleFunc("POST /api/v1/identity/service-token", test.identityServiceToken)
	services := httptest.NewServer(mux)
	MailOutTestSetup{}.Use(mailout.Settings{}, services.URL)
	mailout.Registry{Store: test.store()}.Register(mailout.Queue{}.Base(), test.domain())
	return services
}

func (test *OutboxDueTest) dropQueue() {
	DataSource{}.DropCollection(test.collection())
	collections.KnownCollections{}.Forget(test.collection())
}

func (test *OutboxDueTest) identityServiceToken(writer http.ResponseWriter, request *http.Request) {
	asked := oidc.ServiceTokenRequest{}
	json.NewDecoder(request.Body).Decode(&asked)
	json.NewEncoder(writer).Encode(oidc.ServiceTokenAnswer{AccessToken: MailOutTestSetup{}.ServiceToken(test.realm.ForDomain(asked.DomainId)), ExpiresIn: 60})
}

func (test *OutboxDueTest) messagingEvents(writer http.ResponseWriter, request *http.Request) {
	envelope := events.Envelope{}
	json.NewDecoder(request.Body).Decode(&envelope)
	test.lock.Lock()
	defer test.lock.Unlock()
	test.pushed = append(test.pushed, envelope.EventID)
	writer.WriteHeader(http.StatusOK)
}

func (*OutboxDueTest) store() mailout.DocumentStore {
	return mailout.DocumentStore{Documents: DataSource{}}
}

func (test *OutboxDueTest) collection() string {
	return test.store().Name(mailout.Queue{}.Base(), test.domain())
}

func (*OutboxDueTest) inAnHour() string {
	return time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
}

func (*OutboxDueTest) aMinuteAgo() string {
	return time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
}

func (test *OutboxDueTest) insertMany(count int, nextAttemptAt string) []mailout.Item {
	items := []mailout.Item{}
	for range count {
		items = append(items, test.insert(nextAttemptAt))
	}
	return items
}

func (test *OutboxDueTest) insert(nextAttemptAt string) mailout.Item {
	item := mailout.Item{DomainId: test.domain(), MessageId: "msg_due_order", Rcpt: "bob@partner.test", State: mailout.States{}.Sent()}
	envelope, _ := mailout.ChangeEvents{Now: time.Now}.Envelope(item)
	pending := outbox.PendingEvent{Envelope: envelope, Waiting: []string{test.subscriber()}, Attempts: test.firstAttempts(), NextAttemptAt: nextAttemptAt}
	fields, _ := outbox.Codec{}.Write(outbox.OutboxState{Pending: []outbox.PendingEvent{pending}})
	item = item.WithOutbox(fields)
	item.EventId = envelope.EventID
	item.Id, _ = test.store().Insert(test.collection(), item)
	return item
}

func (test *OutboxDueTest) relayUntilPushed(item mailout.Item) error {
	desk := MailOutTestSetup{}.Desks().Desk()
	desk.Limit = test.batchLimit()
	failures := []error{}
	for round := 0; round < test.relayRounds() && !test.wasPushed(item); round++ {
		_, failure := desk.Relay()
		failures = append(failures, failure)
	}
	return errors.Join(failures...)
}

func (test *OutboxDueTest) wasPushed(item mailout.Item) bool {
	test.lock.Lock()
	defer test.lock.Unlock()
	return slices.Contains(test.pushed, item.EventId)
}

func (test *OutboxDueTest) pendingOf(item mailout.Item) []outbox.PendingEvent {
	ref := outbox.Ref{Collection: test.collection(), Id: item.Id}
	fields, _ := mailout.OutboxStore{Store: test.store()}.Load(ref)
	state, _ := outbox.Codec{}.Read(fields)
	return state.Pending
}

func (test *OutboxDueTest) attemptsAndTimes(items []mailout.Item) []string {
	lines := []string{}
	for _, item := range items {
		lines = append(lines, test.linesOf(test.pendingOf(item))...)
	}
	return lines
}

func (*OutboxDueTest) linesOf(pending []outbox.PendingEvent) []string {
	lines := []string{}
	for _, event := range pending {
		lines = append(lines, fmt.Sprint(event.Attempts, " ", event.NextAttemptAt))
	}
	return lines
}

func (test *OutboxDueTest) untouched(count int, nextAttemptAt string) []string {
	return slices.Repeat([]string{fmt.Sprint(test.firstAttempts(), " ", nextAttemptAt)}, count)
}

func (test *OutboxDueTest) TestADueDocumentIsRelayedPastMoreNotDueDocumentsThanTheBatchLimit() {
	services := test.start()
	defer services.Close()
	defer test.dropQueue()
	later := test.inAnHour()
	notDue := test.insertMany(test.notDueCount(), later)
	due := test.insert(test.aMinuteAgo())

	err := test.relayUntilPushed(due)

	Assert.Equals(nil, err)
	Assert.Equals(true, test.wasPushed(due))
	Assert.Equals(0, len(test.pendingOf(due)))
	Assert.Equals(test.untouched(test.notDueCount(), later), test.attemptsAndTimes(notDue))
}
