import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mesail.dev/packages/collections"
	"mesail.dev/packages/events"
	"mesail.dev/packages/mailout"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/smtpclient/sink"
)

// @Test(MailOutTestSetup)
type MailOutRoutesTest struct {
	behaviors   *BehaviorContext
	folder      string
	lock        sync.Mutex
	handOffs    []mailout.InboundEnvelope
	answers     []int
	eventAnswer int
	pushed      []events.Envelope
	calls       []string
	realm       oidc.SigningRealm
	sources     map[string]mailout.MessageSource
}

func (test *MailOutRoutesTest) start() (*httptest.Server, *sink.Sink) {
	test.realm = MailOutTestSetup{}.Realm()
	test.folder, _ = os.MkdirTemp("", "mail-out-routes-")
	test.handOffs, test.answers, test.pushed, test.calls, test.eventAnswer = nil, nil, nil, nil, http.StatusOK
	test.sources = map[string]mailout.MessageSource{}
	DataSource{}.DropCollection("outbox-collections")
	for _, domainId := range []string{"example.com", "example.org", "partner.test"} {
		for _, base := range []string{"outbound-queue", "suppressions", "delivery-attempts", "events-seen"} {
			DataSource{}.DropCollection(collections.CollectionName{}.ForDomain(base, domainId))
		}
		test.writeKey(domainId)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/mail-in/messages", test.mailIn)
	mux.HandleFunc("GET /api/v1/directory/domains/{domainId}", test.directory)
	mux.HandleFunc("GET /api/v1/{producer}/messages/{messageId}/source", test.source)
	mux.HandleFunc("POST /api/v1/messaging/events", test.messagingEvents)
	mux.HandleFunc("POST /api/v1/identity/service-token", test.identityServiceToken)
	services := httptest.NewServer(mux)
	relay, _ := sink.Start(sink.Options{})
	MailOutTestSetup{}.Use(mailout.Settings{
		MailInUrl: services.URL, MessagingUrl: services.URL, NotifyUrl: services.URL, DirectoryUrl: services.URL, RelayAddress: relay.Address(),
		SecretFolder: filepath.Join(test.folder, "secrets"), BlobFolder: filepath.Join(test.folder, "blobs"),
	}, services.URL)
	return services, relay
}

func (test *MailOutRoutesTest) writeKey(domainId string) {
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	encoded, _ := x509.MarshalPKCS8PrivateKey(private)
	path := filepath.Join(test.folder, "secrets", "dkim", domainId, "mesail.pem")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE" + " KEY", Bytes: encoded}), 0o600)
}

func (test *MailOutRoutesTest) record(name string, request *http.Request) {
	caller, _ := oidc.Configured().Identify(request.Header.Get("Authorization"))
	test.lock.Lock()
	defer test.lock.Unlock()
	test.calls = append(test.calls, name+" "+caller.DomainId+" "+strings.Join(caller.Roles, ","))
}

func (test *MailOutRoutesTest) identityServiceToken(writer http.ResponseWriter, request *http.Request) {
	asked := oidc.ServiceTokenRequest{}
	json.NewDecoder(request.Body).Decode(&asked)
	json.NewEncoder(writer).Encode(oidc.ServiceTokenAnswer{AccessToken: MailOutTestSetup{}.ServiceToken(test.realm.ForDomain(asked.DomainId)), ExpiresIn: 60})
}

func (test *MailOutRoutesTest) mailIn(writer http.ResponseWriter, request *http.Request) {
	test.record("mail-in", request)
	envelope := mailout.InboundEnvelope{}
	json.NewDecoder(request.Body).Decode(&envelope)
	test.lock.Lock()
	defer test.lock.Unlock()
	test.handOffs = append(test.handOffs, envelope)
	writer.WriteHeader(http.StatusCreated)
}

func (test *MailOutRoutesTest) directory(writer http.ResponseWriter, request *http.Request) {
	test.record("directory "+request.PathValue("domainId"), request)
	domainId := request.PathValue("domainId")
	if domainId != "example.com" && domainId != "example.org" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(writer).Encode(mailout.Domain{DomainId: domainId, Name: domainId, State: "active", DkimSelector: "mesail"})
}

func (test *MailOutRoutesTest) source(writer http.ResponseWriter, request *http.Request) {
	test.record("source", request)
	test.lock.Lock()
	source := test.sources[request.PathValue("messageId")]
	test.lock.Unlock()
	json.NewEncoder(writer).Encode(source)
}

func (test *MailOutRoutesTest) messagingEvents(writer http.ResponseWriter, request *http.Request) {
	test.record("push", request)
	envelope := events.Envelope{}
	json.NewDecoder(request.Body).Decode(&envelope)
	test.lock.Lock()
	defer test.lock.Unlock()
	test.pushed = append(test.pushed, envelope)
	writer.WriteHeader(test.eventAnswer)
}

func (test *MailOutRoutesTest) event(domainId string, from string, rcpt string) SendEnvelope {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	key := "k-" + hex.EncodeToString(bytes)
	test.lock.Lock()
	test.sources["msg_"+key] = mailout.MessageSource{MessageId: "msg_" + key, DomainId: domainId, Body: "Hello"}
	test.lock.Unlock()
	request := SendRequested{MessageId: "msg_" + key, From: from, Rcpt: rcpt, Role: "to", To: []string{rcpt}, Cc: []string{}, Subject: "Lunch", References: []string{}, AttachmentIds: []string{}, IdempotencyKey: key}
	return SendEnvelope{EventId: "evt_" + key, Type: "email.send.requested", DomainId: domainId, Producer: "messaging", OccurredAt: time.Now().UTC().Format(time.RFC3339), Payload: request}
}

func (test *MailOutRoutesTest) bearer(domainId string) string {
	return "Bearer " + MailOutTestSetup{}.ServiceToken(test.realm.ForDomain(domainId))
}

func (test *MailOutRoutesTest) recorded() []string {
	test.lock.Lock()
	defer test.lock.Unlock()
	return append([]string{}, test.calls...)
}

func (test *MailOutRoutesTest) TestAHostedRecipientIsHandedToMailInThroughTheRoutes() {
	services, relay := test.start()
	defer services.Close()
	defer relay.Close()
	events, jobs, outboxRoute := &EventsController{}, &JobsController{}, &OutboxController{}
	envelope := test.event("example.com", "Alice <alice@example.com>", "eve@example.org")

	receipt, err := events.PostEvent(test.bearer("example.com"), envelope)
	report, deliverErr := jobs.PostDeliver()
	pending, pendingErr := outboxRoute.GetPending()

	Assert.Equals(nil, err)
	Assert.Equals(EventReceipt{EventId: envelope.EventId, Status: "applied"}, receipt)
	Assert.Equals(nil, deliverErr)
	Assert.Equals(QueueReport{Picked: 1, Sent: 1}, report)
	Assert.Equals(nil, pendingErr)
	Assert.Equals(0, len(pending))
	Assert.Equals(1, len(test.handOffs))
	Assert.Equals("example.com/outbound-", test.handOffs[0].RawBlobKey[:len("example.com/outbound-")])
	Assert.Equals(true, test.handOffs[0].AuthResults.Handoff)
	Assert.Equals([]string{
		"push example.com Service",
		"directory example.com example.com Service",
		"source example.com Service",
		"directory example.org * Service",
		"mail-in example.org Service",
		"push example.com Service",
	}, test.recorded())
	Assert.Equals(0, len(relay.Messages()))
}

func (test *MailOutRoutesTest) TestTheEventsRouteRefusesBadCallersAndEvents() {
	services, relay := test.start()
	defer services.Close()
	defer relay.Close()
	events := &EventsController{}
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test")
	foreignSender := test.event("example.com", "mallory@example.org", "bob@partner.test")

	_, withoutToken := events.PostEvent("", envelope)
	_, otherDomain := events.PostEvent(test.bearer("example.org"), envelope)
	_, senderErr := events.PostEvent(test.bearer("example.com"), foreignSender)

	Assert.Equals(&ForbiddenCaller{}, withoutToken)
	Assert.Equals(&UnknownDomain{}, otherDomain)
	Assert.IsTrue(strings.Contains(senderErr.Error(), "from is missing or not valid"), senderErr.Error())
	Assert.Equals(0, len(test.pushed))
}

func (test *MailOutRoutesTest) TestTheOutboxRoutesRelayAndRequeueEvents() {
	services, relay := test.start()
	defer services.Close()
	defer relay.Close()
	test.eventAnswer = http.StatusBadRequest
	events, outboxRoute := &EventsController{}, &OutboxController{}
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test")
	events.PostEvent(test.bearer("example.com"), envelope)

	dead, deadErr := outboxRoute.GetDead()
	test.eventAnswer = http.StatusOK
	item, _, _ := mailout.Queue{Store: mailout.DocumentStore{Documents: DataSource{}}}.FindBy("example.com", "eventId", envelope.EventId)
	report, requeueErr := outboxRoute.PostRequeue(RequeueRequest{DomainId: "example.com", ItemId: item.Id, EventId: dead[0].EventId, Subscriber: "messaging"})
	relayed, relayErr := outboxRoute.PostRelay()

	Assert.Equals(nil, deadErr)
	Assert.Equals(1, len(dead))
	Assert.Equals("messaging", dead[0].Subscriber)
	Assert.Equals(nil, requeueErr)
	Assert.Equals(RelayReport{Documents: 1, Delivered: 1}, report)
	Assert.Equals(nil, relayErr)
	Assert.Equals(RelayReport{}, relayed)
}
