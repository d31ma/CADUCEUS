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

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/smtpclient/sink"
)

// @Test(MailOutTestSetup)
type OutboundQueueTest struct {
	behaviors   *BehaviorContext
	folder      string
	lock        sync.Mutex
	handOffs    []InboundEnvelope
	headers     []string
	answers     []int
	eventAnswer int
	pushed      []events.Envelope
	domains     map[string]Domain
	sources     map[string]MessageSource
	rejecting   bool
	realms      map[string]oidc.SigningRealm
	calls       []string
}

func (test *OutboundQueueTest) start() {
	test.realms = MailOutTestSetup{}.Realms("example.com", "example.org", "mesail-operators")
	test.folder, _ = os.MkdirTemp("", "mail-out-test-")
	test.handOffs, test.headers, test.answers, test.pushed, test.calls = nil, nil, nil, nil, nil
	test.eventAnswer, test.rejecting = http.StatusOK, true
	test.sources = map[string]MessageSource{}
	test.domains = map[string]Domain{
		"example.com": {DomainId: "example.com", Name: "example.com", State: "active", DkimSelector: "mesail"},
		"example.org": {DomainId: "example.org", Name: "example.org", State: "active", DkimSelector: "mesail"},
	}
	DataSource{}.DropCollection("outbox-collections")
	for _, domainId := range []string{"example.com", "example.org", "partner.test", "paused.test"} {
		for _, collection := range []string{"outbound-queue", "suppressions", "delivery-attempts", "events-seen"} {
			DataSource{}.DropCollection(StorageUtils{}.CollectionName(collection, domainId))
		}
		test.writeKey(domainId)
	}
}

func (test *OutboundQueueTest) writeKey(domainId string) {
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	encoded, _ := x509.MarshalPKCS8PrivateKey(private)
	path := filepath.Join(test.folder, "secrets", "dkim", domainId, "mesail.pem")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE" + " KEY", Bytes: encoded}), 0o600)
}

func (test *OutboundQueueTest) services() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/mail-in/messages", test.mailIn)
	mux.HandleFunc("GET /api/v1/directory/domains/{domainId}", test.directory)
	mux.HandleFunc("GET /api/v1/{producer}/messages/{messageId}/source", test.source)
	mux.HandleFunc("POST /api/v1/messaging/events", test.messagingEvents)
	mux.HandleFunc("POST /realms/{realm}/protocol/openid-connect/token", test.keycloak)
	return httptest.NewServer(mux)
}

func (test *OutboundQueueTest) keycloak(writer http.ResponseWriter, request *http.Request) {
	realm, found := test.realms[request.PathValue("realm")]
	if !found {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	json.NewEncoder(writer).Encode(map[string]any{"access_token": MailOutTestSetup{}.ServiceToken(realm), "expires_in": 60})
}

func (test *OutboundQueueTest) record(name string, request *http.Request) {
	caller, err := oidc.Configured().Identify(request.Header.Get("Authorization"))
	test.lock.Lock()
	defer test.lock.Unlock()
	test.calls = append(test.calls, name+" "+caller.DomainId+" "+strings.Join(caller.Roles, ",")+test.failure(err))
}

func (test *OutboundQueueTest) failure(err error) string {
	if err != nil {
		return " " + err.Error()
	}
	return ""
}

func (test *OutboundQueueTest) recorded() []string {
	test.lock.Lock()
	defer test.lock.Unlock()
	return append([]string{}, test.calls...)
}

func (test *OutboundQueueTest) mailIn(writer http.ResponseWriter, request *http.Request) {
	test.record("mail-in", request)
	envelope := InboundEnvelope{}
	json.NewDecoder(request.Body).Decode(&envelope)
	test.lock.Lock()
	defer test.lock.Unlock()
	test.handOffs = append(test.handOffs, envelope)
	test.headers = append(test.headers, request.Header.Get("Authorization"))
	answer := test.answers[0]
	test.answers = test.answers[1:]
	writer.WriteHeader(answer)
}

func (test *OutboundQueueTest) directory(writer http.ResponseWriter, request *http.Request) {
	test.record("directory "+request.PathValue("domainId"), request)
	test.lock.Lock()
	domain, found := test.domains[request.PathValue("domainId")]
	test.lock.Unlock()
	if !found {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(writer).Encode(domain)
}

func (test *OutboundQueueTest) source(writer http.ResponseWriter, request *http.Request) {
	test.record("source "+request.PathValue("producer"), request)
	test.lock.Lock()
	source, found := test.sources[request.PathValue("producer")+" "+request.PathValue("messageId")]
	test.lock.Unlock()
	if !found {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(writer).Encode(source)
}

func (test *OutboundQueueTest) messagingEvents(writer http.ResponseWriter, request *http.Request) {
	test.record("push", request)
	envelope := events.Envelope{}
	json.NewDecoder(request.Body).Decode(&envelope)
	test.lock.Lock()
	defer test.lock.Unlock()
	test.pushed = append(test.pushed, envelope)
	writer.WriteHeader(test.eventAnswer)
}

func (test *OutboundQueueTest) relay(rejected string, code int, text string) *sink.Sink {
	relay, _ := sink.Start(sink.Options{RejectRecipient: func(address string) (int, string) {
		test.lock.Lock()
		defer test.lock.Unlock()
		if address != rejected || !test.rejecting {
			return 0, ""
		}
		return code, text
	}})
	return relay
}

func (test *OutboundQueueTest) settings(services *httptest.Server, relay *sink.Sink) MailOutSettings {
	return MailOutSettings{
		MailInUrl: services.URL, MessagingUrl: services.URL, NotifyUrl: services.URL, DirectoryUrl: services.URL,
		RelayAddress: relay.Address(),
		SecretFolder: filepath.Join(test.folder, "secrets"), BlobFolder: filepath.Join(test.folder, "blobs"), MaxAgeMinutes: 7200,
	}
}

func (test *OutboundQueueTest) wire(settings MailOutSettings, services *httptest.Server) (*EventsController, *JobsController, *QueueService, *OutboxController) {
	MailOutTestSetup{}.Use(settings, services.URL+"/api/v1/messaging/events", services.URL)
	config := &SettingsDelegate{}
	client := &ServiceClientDelegate{settings: config}
	registry := &CollectionRegistryDelegate{}
	directory := &DirectoryDelegate{settings: config, client: client}
	blobs := &BlobStoreDelegate{settings: config}
	sources := &MessageSourceDelegate{settings: config, client: client}
	outbound := &OutboundStoreDelegate{registry: registry, settings: config}
	suppressions := &SuppressionStoreDelegate{}
	relay := &OutboxRelayService{store: &OutboxStoreDelegate{registry: registry}, settings: config}
	builder := &MessageBuilderService{sources: sources, attachments: &AttachmentSourceDelegate{blobs: blobs}, blobs: blobs, keys: &DkimKeyDelegate{settings: config}, directory: directory}
	mailIn := &MailInClientDelegate{settings: config, client: client}
	transport := &TransportService{directory: directory, mailIn: mailIn, relay: &SmtpRelayDelegate{settings: config}, direct: &SmtpDirectDelegate{settings: config}, settings: config}
	outcomes := &OutcomeService{outbound: outbound, suppressions: suppressions, attempts: &AttemptStoreDelegate{}, relay: relay, settings: config}
	delivery := &DeliveryService{outbound: outbound, suppressions: suppressions, builder: builder, transport: transport, outcomes: outcomes}
	queue := &QueueService{registry: registry, outbound: outbound, delivery: delivery}
	intake := &EventIntakeService{outbound: outbound, relay: relay, settings: config, sources: sources}
	events := &EventsService{seenEvents: &SeenEventsDelegate{}, intake: intake}
	return &EventsController{events: events}, &JobsController{queue: queue}, queue, &OutboxController{relay: relay}
}

func (test *OutboundQueueTest) bearer(realm string) string {
	return "Bearer " + MailOutTestSetup{}.ServiceToken(test.realms[realm])
}

func (test *OutboundQueueTest) authorize() func(string) (string, error) {
	return oidc.ServiceTokensFactory{}.Configured().Authorization
}

func (test *OutboundQueueTest) send(events *EventsController, envelope SendEnvelope) (EventReceipt, error) {
	return events.PostEvent(test.bearer(envelope.DomainId), envelope)
}

func (test *OutboundQueueTest) unique() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (test *OutboundQueueTest) event(domainId string, from string, rcpt string, key string) SendEnvelope {
	messageId := "msg_" + key
	test.lock.Lock()
	test.sources["messaging "+messageId] = MessageSource{MessageId: messageId, DomainId: domainId, Body: "Hello Eve,\n\nsee you **soon**.\n"}
	test.lock.Unlock()
	request := SendRequested{
		MessageId: messageId, From: from, Rcpt: rcpt, Role: "to", To: []string{rcpt}, Cc: []string{}, Subject: "Lunch on Friday",
		InReplyTo: "<parent@example.org>", References: []string{"<root@example.org>", "<parent@example.org>"},
		AttachmentIds: []string{}, IdempotencyKey: key,
	}
	return SendEnvelope{EventId: "evt_" + test.unique(), Type: "email.send.requested", DomainId: domainId, Producer: "messaging", OccurredAt: TimeUtils{}.Format(time.Now()), Payload: request}
}

func (test *OutboundQueueTest) changes() []string {
	test.lock.Lock()
	defer test.lock.Unlock()
	statuses := []string{}
	for _, envelope := range test.pushed {
		change := events.EmailDeliveryChanged{}
		json.Unmarshal(envelope.Payload, &change)
		statuses = append(statuses, envelope.Producer+" "+envelope.Type+" "+envelope.DomainId+" "+change.Rcpt+" "+change.Status+" "+change.Detail)
	}
	return statuses
}

func (test *OutboundQueueTest) item(domainId string, eventId string) OutboundItem {
	item, _, _ := (&OutboundStoreDelegate{}).FindBy(domainId, "eventId", eventId)
	return item
}

func (test *OutboundQueueTest) raw(item OutboundItem) string {
	raw, _ := (&BlobStoreDelegate{settings: &SettingsDelegate{}}).Get(item.DomainId, item.RawBlobKey)
	return string(raw)
}

func (test *OutboundQueueTest) header(raw string, name string) string {
	headers, _, _ := strings.Cut(raw, "\r\n\r\n")
	for _, line := range strings.Split(strings.ReplaceAll(strings.ReplaceAll(headers, "\r\n\t", " "), "\r\n ", " "), "\r\n") {
		field, value, _ := strings.Cut(line, ":")
		if strings.EqualFold(field, name) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (test *OutboundQueueTest) TestHostedRecipientIsHandedToMailInSignedAndThreaded() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	test.answers = []int{503, 201}
	settings := test.settings(services, relay)
	events, jobs, queue, _ := test.wire(settings, services)
	envelope := test.event("example.com", "Alice <alice@example.com>", "eve@example.org", "k-"+test.unique())
	envelope.Payload.To = []string{"eve@example.org", "Bob <bob@partner.test>"}
	envelope.Payload.Cc = []string{"carol@example.org"}

	receipt, err := test.send(events, envelope)
	Assert.Equals(nil, err)
	Assert.Equals(EventReceipt{EventId: envelope.EventId, Status: "applied"}, receipt)
	first, err := jobs.PostDeliver()
	Assert.Equals(nil, err)
	Assert.Equals(QueueReport{Picked: 1, Deferred: 1}, first)
	Assert.Equals("deferred", test.item("example.com", envelope.EventId).State)
	second, err := queue.Run(time.Now().Add(2*time.Minute), test.authorize())
	Assert.Equals(nil, err)
	Assert.Equals(QueueReport{Picked: 1, Sent: 1}, second)

	item := test.item("example.com", envelope.EventId)
	raw := test.raw(item)
	Assert.Equals("sent", item.State)
	Assert.Equals("example.com/outbound-"+item.Id, item.RawBlobKey)
	Assert.Equals(2, len(test.handOffs))
	Assert.Equals(InboundEnvelope{
		EnvelopeFrom: "alice@example.com", Recipients: []string{"eve@example.org"}, RawBlobKey: item.RawBlobKey, Size: int64(len(raw)),
		RemoteAddress: "internal", Helo: "mail-out", Tls: true,
		Headers: InboundHeaders{
			MessageId: envelope.Payload.MessageId + "@example.com", InReplyTo: "parent@example.org", References: []string{"root@example.org", "parent@example.org"},
			Subject: "Lunch on Friday", From: MailboxAddress{Name: "Alice", Address: "alice@example.com"},
			To: []MailboxAddress{{Address: "eve@example.org"}, {Name: "Bob", Address: "bob@partner.test"}}, Cc: []MailboxAddress{{Address: "carol@example.org"}},
		},
		AuthResults: AuthResults{DmarcPolicy: "none", Handoff: true},
	}, test.handOffs[1])
	Assert.Equals([]string{
		"push example.com Service",
		"directory example.com example.com Service",
		"source messaging example.com Service",
		"directory example.org mesail-operators Service",
		"mail-in example.org Service",
		"push example.com Service",
		"directory example.com example.com Service",
		"source messaging example.com Service",
		"directory example.org mesail-operators Service",
		"mail-in example.org Service",
		"push example.com Service",
	}, test.recorded())
	Assert.Equals(0, len(relay.Messages()), "a hosted recipient never goes through SMTP")
	Assert.IsTrue(strings.HasPrefix(raw, "DKIM-Signature:"), "the raw message starts with its signature")
	Assert.IsTrue(strings.Contains(test.header(raw, "DKIM-Signature"), "d=example.com;"), test.header(raw, "DKIM-Signature"))
	Assert.IsTrue(strings.Contains(test.header(raw, "DKIM-Signature"), "s=mesail;"), test.header(raw, "DKIM-Signature"))
	Assert.Equals("<parent@example.org>", test.header(raw, "In-Reply-To"))
	Assert.Equals("<root@example.org> <parent@example.org>", test.header(raw, "References"))
	Assert.Equals("<"+envelope.Payload.MessageId+"@example.com>", test.header(raw, "Message-ID"))
	Assert.IsTrue(strings.Contains(test.header(raw, "To"), "bob@partner.test"), test.header(raw, "To"))
	Assert.IsTrue(strings.Contains(test.header(raw, "Cc"), "carol@example.org"), test.header(raw, "Cc"))
	Assert.Equals("", test.header(raw, "Bcc"))
	Assert.Equals("", test.header(raw, "Authentication-Results"))
	Assert.IsTrue(strings.Contains(raw, "see you **soon**."), "the Markdown body is sent as plain text")
	Assert.Equals([]string{
		"mail-out email.delivery.changed example.com eve@example.org queued ",
		"mail-out email.delivery.changed example.com eve@example.org deferred mail-in answered 503",
		"mail-out email.delivery.changed example.com eve@example.org sent accepted by mail-in",
	}, test.changes())
	Assert.Equals(0, item.OutboxPending)
}

func (test *OutboundQueueTest) TestBccCopyGoesToTheRelayWithoutTheBccAddressInHeaders() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	events, jobs, _, _ := test.wire(test.settings(services, relay), services)
	envelope := test.event("example.com", "alice@example.com", "dave@partner.test", "k-"+test.unique())
	envelope.Payload.Role = "bcc"
	envelope.Payload.To = []string{"eve@example.org"}
	test.send(events, envelope)

	report, err := jobs.PostDeliver()
	data := string(relay.Messages()[0].Data)

	Assert.Equals(nil, err)
	Assert.Equals(QueueReport{Picked: 1, Sent: 1}, report)
	Assert.Equals([]string{"dave@partner.test"}, relay.Messages()[0].Recipients)
	Assert.Equals("alice@example.com", relay.Messages()[0].From)
	Assert.IsTrue(strings.Contains(test.header(data, "To"), "eve@example.org"), test.header(data, "To"))
	Assert.IsTrue(!strings.Contains(strings.SplitN(data, "\r\n\r\n", 2)[0], "dave@partner.test"), "the Bcc address is in no header")
	Assert.IsTrue(strings.Contains(test.header(data, "DKIM-Signature"), "d=example.com;"))
	Assert.Equals(0, len(test.handOffs))
}

func (test *OutboundQueueTest) TestTemporaryReplyDefersWithBackoff() {
	test.start()
	services, relay := test.services(), test.relay("bob@partner.test", 451, "4.3.0 try again later")
	defer services.Close()
	defer relay.Close()
	events, _, queue, _ := test.wire(test.settings(services, relay), services)
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique())
	now := time.Now()
	test.send(events, envelope)

	first, _ := queue.Run(now, test.authorize())
	item := test.item("example.com", envelope.EventId)
	early, _ := queue.Run(now.Add(30*time.Second), test.authorize())
	second, _ := queue.Run(now.Add(61*time.Second), test.authorize())
	later := test.item("example.com", envelope.EventId)
	attempts, err := (&AttemptStoreDelegate{}).forItem("example.com", item.Id)

	Assert.Equals(QueueReport{Picked: 1, Deferred: 1}, first)
	Assert.Equals(TimeUtils{}.Format(now.Add(time.Minute)), item.NextAttemptAt)
	Assert.Equals(QueueReport{}, early)
	Assert.Equals(QueueReport{Picked: 1, Deferred: 1}, second)
	Assert.Equals(TimeUtils{}.Format(now.Add(61*time.Second).Add(2*time.Minute)), later.NextAttemptAt)
	Assert.Equals(2, later.Attempts)
	Assert.Equals(nil, err)
	Assert.Equals(2, len(attempts))
	Assert.Equals("4.3.0", attempts[0].Enhanced)
	Assert.Equals(451, attempts[0].Code)
}

func (test *OutboundQueueTest) TestUnknownMailboxBouncesAndIsSuppressedOnlyInTheSendersDomain() {
	test.start()
	services, relay := test.services(), test.relay("gone@partner.test", 550, "5.1.1 no such user")
	defer services.Close()
	defer relay.Close()
	events, _, queue, _ := test.wire(test.settings(services, relay), services)
	suppressions := &SuppressionStoreDelegate{}
	bounced := test.event("example.com", "alice@example.com", "gone@partner.test", "k-"+test.unique())
	again := test.event("example.com", "alice@example.com", "gone@partner.test", "k-"+test.unique())
	other := test.event("example.org", "eve@example.org", "gone@partner.test", "k-"+test.unique())
	now := time.Now()

	test.send(events, bounced)
	first, _ := queue.Run(now, test.authorize())
	inSender, _ := suppressions.Contains("example.com", "GONE@partner.test")
	inOther, _ := suppressions.Contains("example.org", "gone@partner.test")
	test.rejecting = false
	test.send(events, again)
	test.send(events, other)
	second, _ := queue.Run(now.Add(time.Second), test.authorize())

	Assert.Equals(QueueReport{Picked: 1, Bounced: 1}, first)
	Assert.Equals("bounced", test.item("example.com", bounced.EventId).State)
	Assert.IsTrue(inSender, "the sender's domain suppresses the address")
	Assert.IsTrue(!inOther, "another domain's list is untouched")
	Assert.Equals(QueueReport{Picked: 2, Sent: 1, Failed: 1}, second)
	Assert.Equals("suppressed", test.item("example.com", again.EventId).Detail)
	Assert.Equals("sent", test.item("example.org", other.EventId).State)
	Assert.Equals("eve@example.org", relay.Messages()[0].From)
}

func (test *OutboundQueueTest) TestItemExpiresAfterTheMaxAge() {
	test.start()
	services, relay := test.services(), test.relay("bob@partner.test", 421, "4.4.2 busy")
	defer services.Close()
	defer relay.Close()
	settings := test.settings(services, relay)
	settings.MaxAgeMinutes = 10
	events, _, queue, _ := test.wire(settings, services)
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique())
	now := time.Now()

	test.send(events, envelope)
	first, _ := queue.Run(now, test.authorize())
	late, _ := queue.Run(now.Add(11*time.Minute), test.authorize())
	statuses := test.changes()

	Assert.Equals(QueueReport{Picked: 1, Deferred: 1}, first)
	Assert.Equals(QueueReport{Picked: 1, Expired: 1}, late)
	Assert.Equals("expired", test.item("example.com", envelope.EventId).State)
	Assert.Equals("mail-out email.delivery.changed example.com bob@partner.test failed expired", statuses[len(statuses)-1])
}

func (test *OutboundQueueTest) TestRepeatedEventAndIdempotencyKeyMakeOneItem() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	events, jobs, _, _ := test.wire(test.settings(services, relay), services)
	key := "k-" + test.unique()
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test", key)
	sameKey := test.event("example.com", "alice@example.com", "bob@partner.test", key)

	first, _ := test.send(events, envelope)
	repeated, repeatedErr := test.send(events, envelope)
	otherEvent, otherErr := test.send(events, sameKey)
	report, _ := jobs.PostDeliver()

	Assert.Equals("applied", first.Status)
	Assert.Equals(nil, repeatedErr)
	Assert.Equals(EventReceipt{EventId: envelope.EventId, Status: "duplicate"}, repeated)
	Assert.Equals(nil, otherErr)
	Assert.Equals("applied", otherEvent.Status)
	Assert.Equals(QueueReport{Picked: 1, Sent: 1}, report)
	Assert.Equals(1, len(relay.Messages()))
	Assert.Equals(2, len(test.changes()))
}

func (test *OutboundQueueTest) TestDirectDeliveryUsesTheMxHost() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	settings := test.settings(services, relay)
	settings.DirectEnabled, settings.DirectMxHost, settings.DirectPort, settings.RelayAddress = true, "127.0.0.1", relay.Port(), "relay.invalid:25"
	events, jobs, _, _ := test.wire(settings, services)
	test.send(events, test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique()))

	report, err := jobs.PostDeliver()

	Assert.Equals(nil, err)
	Assert.Equals(QueueReport{Picked: 1, Sent: 1}, report)
	Assert.Equals([]string{"bob@partner.test"}, relay.Messages()[0].Recipients)
}

func (test *OutboundQueueTest) TestAMissingDkimKeyDefersTheItem() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	test.domains["example.com"] = Domain{DomainId: "example.com", State: "active", DkimSelector: "missing"}
	events, jobs, _, _ := test.wire(test.settings(services, relay), services)
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique())
	test.send(events, envelope)

	report, _ := jobs.PostDeliver()

	Assert.Equals(QueueReport{Picked: 1, Deferred: 1}, report)
	Assert.Equals(0, len(relay.Messages()))
	Assert.IsTrue(!strings.Contains(test.item("example.com", envelope.EventId).Detail, "PRIVATE"), "no key material in the detail")
}

func (test *OutboundQueueTest) TestAHeaderLineBreakFailsTheItem() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	events, jobs, _, _ := test.wire(test.settings(services, relay), services)
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique())
	envelope.Payload.Subject = "Hello\r\nBcc: mallory@partner.test"
	test.send(events, envelope)

	report, _ := jobs.PostDeliver()

	Assert.Equals(QueueReport{Picked: 1, Failed: 1}, report)
	Assert.Equals(0, len(relay.Messages()))
}

func (test *OutboundQueueTest) TestAnInactiveSenderDomainIsRefused() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	test.domains["example.com"] = Domain{DomainId: "example.com", State: "disabled", DkimSelector: "mesail"}
	events, jobs, _, _ := test.wire(test.settings(services, relay), services)
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique())
	test.send(events, envelope)

	report, _ := jobs.PostDeliver()

	Assert.Equals(QueueReport{Picked: 1, Failed: 1}, report)
	Assert.Equals("the sender domain is not active", test.item("example.com", envelope.EventId).Detail)
	Assert.Equals(0, len(relay.Messages()))
}

func (test *OutboundQueueTest) TestASourceFromAnotherDomainIsRefused() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	events, jobs, _, _ := test.wire(test.settings(services, relay), services)
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique())
	test.sources["messaging "+envelope.Payload.MessageId] = MessageSource{MessageId: envelope.Payload.MessageId, DomainId: "example.org", Body: "secret"}
	test.send(events, envelope)

	report, _ := jobs.PostDeliver()

	Assert.Equals(QueueReport{Picked: 1, Failed: 1}, report)
	Assert.Equals("the message source belongs to another domain", test.item("example.com", envelope.EventId).Detail)
}

func (test *OutboundQueueTest) TestAnInactiveRecipientDomainGoesToTheRelay() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	test.domains["paused.test"] = Domain{DomainId: "paused.test", State: "pending", DkimSelector: "mesail"}
	events, jobs, _, _ := test.wire(test.settings(services, relay), services)
	test.send(events, test.event("example.com", "alice@example.com", "zoe@paused.test", "k-"+test.unique()))

	report, _ := jobs.PostDeliver()

	Assert.Equals(QueueReport{Picked: 1, Sent: 1}, report)
	Assert.Equals(0, len(test.handOffs))
	Assert.Equals(1, len(relay.Messages()))
}

func (test *OutboundQueueTest) TestNotifyIsASourceProducer() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	_, _, queue, _ := test.wire(test.settings(services, relay), services)
	test.sources["notify msg_summary"] = MessageSource{MessageId: "msg_summary", DomainId: "example.com", Body: "Your summary"}

	source, err := queue.delivery.builder.sources.Source(OutboundItem{DomainId: "example.com", Producer: "notify", MessageId: "msg_summary"}, test.authorize())
	_, unknownErr := queue.delivery.builder.sources.Source(OutboundItem{DomainId: "example.com", Producer: "search", MessageId: "msg_summary"}, test.authorize())

	Assert.Equals(nil, err)
	Assert.Equals("Your summary", source.Body)
	Assert.Equals([]string{"source notify example.com Service"}, test.recorded())
	Assert.Equals(`no message source is known for the producer "search"`, unknownErr.Error())
}

func (test *OutboundQueueTest) TestTheEventsRouteRefusesBadCallersAndEvents() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	events, _, _, _ := test.wire(test.settings(services, relay), services)
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique())
	foreignSender := test.event("example.com", "mallory@example.org", "bob@partner.test", "k-"+test.unique())
	wrongProducer := test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique())
	wrongProducer.Producer = "channels"
	member, _ := test.realms["example.com"].Token(oidc.TokenRequest{Subject: "member-1", RealmRoles: []string{"mesail-member"}, Audience: "mesail-api", AuthorizedParty: "mesail-web", ExpiresAt: time.Now().Unix() + 60})

	_, withoutToken := events.PostEvent("", envelope)
	_, memberErr := events.PostEvent("Bearer "+member, envelope)
	_, otherRealm := events.PostEvent(test.bearer("example.org"), envelope)
	_, senderErr := test.send(events, foreignSender)
	_, producerErr := test.send(events, wrongProducer)

	Assert.Equals(&ForbiddenCaller{}, withoutToken)
	Assert.Equals(&ForbiddenCaller{}, memberErr)
	Assert.Equals(&UnknownDomain{}, otherRealm)
	Assert.IsTrue(strings.Contains(senderErr.Error(), "from is missing or not valid"), senderErr.Error())
	Assert.IsTrue(strings.Contains(producerErr.Error(), "producer"), producerErr.Error())
	Assert.Equals(0, len(test.changes()))
}

func (test *OutboundQueueTest) TestTheOutboxKeepsEventsUntilMessagingAccepts() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	test.eventAnswer = http.StatusServiceUnavailable
	events, _, _, outboxRoute := test.wire(test.settings(services, relay), services)
	test.send(events, test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique()))

	waiting, pendingErr := outboxRoute.GetPending()
	test.eventAnswer = http.StatusOK
	time.Sleep(1100 * time.Millisecond)
	report, relayErr := outboxRoute.PostRelay()
	after, _ := outboxRoute.GetPending()

	Assert.Equals(nil, pendingErr)
	Assert.Equals(1, len(waiting))
	Assert.Equals(1, waiting[0].OutboxPending)
	Assert.Equals(nil, relayErr)
	Assert.Equals(RelayReport{Documents: 1, Delivered: 1}, report)
	Assert.Equals(0, len(after))
	Assert.Equals(2, len(test.changes()))
}

func (test *OutboundQueueTest) TestADeadLetterCanBeSentAgain() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	test.eventAnswer = http.StatusBadRequest
	events, _, _, outboxRoute := test.wire(test.settings(services, relay), services)
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique())
	test.send(events, envelope)
	item := test.item("example.com", envelope.EventId)

	dead, deadErr := outboxRoute.GetDead()
	test.eventAnswer = http.StatusOK
	report, requeueErr := outboxRoute.PostRequeue(RequeueRequest{DomainId: "example.com", ItemId: item.Id, EventId: dead[0].EventId, Subscriber: "messaging"})
	after, _ := outboxRoute.GetDead()

	Assert.Equals(nil, deadErr)
	Assert.Equals(1, len(dead))
	Assert.Equals("messaging", dead[0].Subscriber)
	Assert.Equals(nil, requeueErr)
	Assert.Equals(1, report.Delivered)
	Assert.Equals(0, len(after))
}

func (test *OutboundQueueTest) TestSavingAnItemKeepsItsStoredOutboxFields() {
	test.start()
	services, relay := test.services(), test.relay("", 0, "")
	defer services.Close()
	defer relay.Close()
	test.eventAnswer = http.StatusServiceUnavailable
	events, _, _, _ := test.wire(test.settings(services, relay), services)
	envelope := test.event("example.com", "alice@example.com", "bob@partner.test", "k-"+test.unique())
	test.send(events, envelope)
	stored := test.item("example.com", envelope.EventId)
	changed := stored
	changed.Detail, changed.PendingEvents, changed.OutboxPending = "checked by an operator", nil, 0

	err := (&OutboundStoreDelegate{registry: &CollectionRegistryDelegate{}, settings: &SettingsDelegate{}}).Save(changed)
	after := test.item("example.com", envelope.EventId)

	Assert.Equals(nil, err)
	Assert.Equals("checked by an operator", after.Detail)
	Assert.Equals(1, after.OutboxPending)
	Assert.Equals(stored.PendingEvents, after.PendingEvents)
}
