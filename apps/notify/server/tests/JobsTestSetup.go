import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/webpush"
)

// @TestSetup
type JobsTestSetup struct{}

func (JobsTestSetup) FreshRegistries() {
	names := &CollectionNamesService{}
	DataSource{}.DropCollection(names.OutboxCollections())
	DataSource{}.DropCollection(names.Domains())
}

func (JobsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (JobsTestSetup) PushServer(status int) (*httptest.Server, *atomic.Int32) {
	calls := &atomic.Int32{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		writer.WriteHeader(status)
	}))
	return server, calls
}

func (JobsTestSetup) ClientFor(server *httptest.Server) *http.Client {
	transport := server.Client().Transport.(*http.Transport).Clone()
	address := server.Listener.Addr().String()
	transport.DialContext = func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func (JobsTestSetup) PresenceServer(states map[string]string, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		answers := []Presence{}
		for _, memberId := range strings.Split(request.URL.Query().Get("memberIds"), ",") {
			answers = append(answers, Presence{MemberId: memberId, State: states[memberId]})
		}
		if !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer ") || request.URL.Path != "/api/v1/realtime/presence" {
			status = http.StatusUnauthorized
		}
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(answers)
	}))
}

func (JobsTestSetup) UseNotify(hosts []string, presenceUrl string) {
	document := map[string]any{}
	if err := json.Unmarshal([]byte(os.Getenv("YON_APPLICATION_JSON")), &document); err != nil {
		panic(err)
	}
	notify := document["yon"].(map[string]any)["notify"].(map[string]any)
	notify["push"] = map[string]any{"enabled": true, "hosts": hosts}
	notify["presence"] = map[string]any{"url": presenceUrl}
	raw, err := json.Marshal(document)
	if err != nil {
		panic(err)
	}
	os.Setenv("YON_APPLICATION_JSON", string(raw))
}

func (setup JobsTestSetup) WithoutVapid() {
	setup.changeNotify(func(notify map[string]any) { delete(notify, "vapid") })
}

func (setup JobsTestSetup) WithPushTurnedOff() {
	setup.changeNotify(func(notify map[string]any) { notify["push"].(map[string]any)["enabled"] = false })
}

func (JobsTestSetup) changeNotify(change func(notify map[string]any)) {
	document := map[string]any{}
	if err := json.Unmarshal([]byte(os.Getenv("YON_APPLICATION_JSON")), &document); err != nil {
		panic(err)
	}
	change(document["yon"].(map[string]any)["notify"].(map[string]any))
	raw, err := json.Marshal(document)
	if err != nil {
		panic(err)
	}
	os.Setenv("YON_APPLICATION_JSON", string(raw))
}

func (JobsTestSetup) Subscribe(domainId string, memberId string, platform string) PushSubscriptionRecord {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		panic(err)
	}
	record := PushSubscriptionRecord{
		DomainId:  domainId,
		MemberId:  memberId,
		Platform:  platform,
		Endpoint:  "https://push.example.com/send/" + strings.ToLower(rand.Text()),
		P256dh:    base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		Auth:      base64.RawURLEncoding.EncodeToString(auth),
		CreatedAt: (&TimestampService{}).Now(),
	}
	store := JobsTestSetup{}.PushStore()
	if err := store.Upsert(record); err != nil {
		panic(err)
	}
	return record
}

func (JobsTestSetup) Stored(record NotificationRecord) NotificationRecord {
	stored, err := JobsTestSetup{}.NotificationStore().InsertIfAbsent(record)
	if err != nil {
		panic(err)
	}
	return stored
}

func (setup JobsTestSetup) StoredMany(domainId string, memberId string, count int, authorDisplayName string) []NotificationRecord {
	records := []NotificationRecord{}
	for range count {
		record := setup.Notification(domainId, memberId, setup.Name("msg"))
		record.AuthorDisplayName = authorDisplayName
		records = append(records, setup.Stored(record))
	}
	return records
}

func (JobsTestSetup) DocumentBytes(document any) int {
	raw, err := json.Marshal(document)
	if err != nil {
		panic(err)
	}
	return len(raw)
}

func (JobsTestSetup) DocumentLimitBytes() int {
	return 512 * 1024
}

func (JobsTestSetup) Found(record NotificationRecord) NotificationRecord {
	found, _, err := JobsTestSetup{}.NotificationStore().Find(record.DomainId, record.Id)
	if err != nil {
		panic(err)
	}
	return found
}

func (JobsTestSetup) MemberSettings(domainId string, memberId string, afterMinutes int, address string) MemberSettingsRecord {
	store := JobsTestSetup{}.SettingsStore()
	preferences := NotifySettings{EmailSummaryAfterMinutes: afterMinutes, MutedConversationIds: []string{}}
	if err := store.PutPreferences(domainId, memberId, preferences); err != nil {
		panic(err)
	}
	profile := MemberProfilePatch{DisplayName: "Ana", PrimaryAddress: address, State: "active"}
	if err := store.PutProfile(domainId, memberId, profile); err != nil {
		panic(err)
	}
	member, err := store.Get(domainId, memberId)
	if err != nil {
		panic(err)
	}
	return member
}

func (JobsTestSetup) Budget(writes int) JobBudget {
	return JobBudget{Writes: writes, Deadline: time.Now().Add(15 * time.Second)}
}

func (JobsTestSetup) RetryLater() *outbox.RecordingPusher {
	pusher := outbox.NewRecordingPusher()
	pusher.FailFor("mail-out", outbox.DeliveryFailure{Subscriber: "mail-out", Status: 503, Permanent: false, Detail: "busy"})
	return pusher
}

func (JobsTestSetup) Targets(client *http.Client) *PushTargetsService {
	return &PushTargetsService{settings: &NotifySettingsService{}, client: client}
}

func (JobsTestSetup) Presence() *PresenceService {
	return &PresenceService{settings: &NotifySettingsService{}}
}

func (JobsTestSetup) Sender() *PushSenderService {
	return &PushSenderService{subscriptions: JobsTestSetup{}.PushStore()}
}

func (setup JobsTestSetup) Pushes(client *http.Client) *PushService {
	return &PushService{store: JobsTestSetup{}.NotificationStore(), presence: setup.Presence(), targets: setup.Targets(client), sender: setup.Sender(), states: &NotificationStatesService{}}
}

func (JobsTestSetup) Emails(pusher outbox.Pusher) *SummaryEmailService {
	notify := JobsTestSetup{}
	return &SummaryEmailService{settings: &NotifySettingsService{}, timestamps: &TimestampService{}, summaries: notify.SummaryStore(), outboxStore: notify.OutboxStore(), relay: notify.Relay(pusher)}
}

func (setup JobsTestSetup) Summaries(pusher outbox.Pusher) *SummaryService {
	notify := JobsTestSetup{}
	return &SummaryService{store: notify.NotificationStore(), members: notify.SettingsStore(), emails: setup.Emails(pusher), timestamps: &TimestampService{}}
}

func (JobsTestSetup) Sources() *MessageSourceService {
	return &MessageSourceService{callers: &CallerService{}, summaries: JobsTestSetup{}.SummaryStore()}
}

func (setup JobsTestSetup) Jobs(client *http.Client, pusher outbox.Pusher) *JobsService {
	return &JobsService{callers: &CallerService{}, domains: setup.Domains(), runner: setup.Runner(client, pusher)}
}

func (setup JobsTestSetup) Runner(client *http.Client, pusher outbox.Pusher) *JobRunService {
	return &JobRunService{fanout: setup.Fanout(), pushes: setup.Pushes(client), summaries: setup.Summaries(pusher)}
}

func (JobsTestSetup) Domain() string {
	return "d" + strings.ToLower(rand.Text())[:12] + ".example.com"
}

func (JobsTestSetup) Name(base string) string {
	return base + "-" + strings.ToLower(rand.Text())[:12]
}

func (setup JobsTestSetup) Realms(domainId string) (oidc.SigningRealm, oidc.SigningRealm) {
	folder, err := os.MkdirTemp("", "notify-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("TMPDIR", folder)
	realm := setup.started()
	setup.Use(realm.Settings)
	return realm.ForDomain(domainId), realm.ForDomain(oidc.InstallWide{}.DomainId())
}

func (JobsTestSetup) started() oidc.SigningRealm {
	realm, err := oidc.SigningRealm{}.StartedMesail()
	if err != nil {
		panic(err)
	}
	if err := realm.Publish(time.Now()); err != nil {
		panic(err)
	}
	return realm
}

func (JobsTestSetup) Use(settings oidc.Settings) {
	keys, err := webpush.VapidKeyGenerator{}.Generate(rand.Reader)
	if err != nil {
		panic(err)
	}
	document := map[string]any{"yon": map[string]any{
		"oidc":    settings,
		"service": map[string]any{"client": map[string]any{"secret": "service-secret"}},
		"outbox": map[string]any{
			"producer":    "notify",
			"subscribers": []any{map[string]any{"name": "mail-out", "url": "http://127.0.0.1:8110/api/v1/mail-out/events"}},
		},
		"notify": map[string]any{
			"push": map[string]any{"enabled": true, "hosts": webpush.EndpointPolicy{}.DefaultHosts()},
			"vapid": map[string]any{
				"public":  map[string]any{"key": keys.PublicKey},
				"private": map[string]any{"key": keys.PrivateKey},
				"subject": "mailto:postmaster@example.com",
			},
			"summary": map[string]any{"sender": "notifications"},
		},
	}}
	raw, err := json.Marshal(document)
	if err != nil {
		panic(err)
	}
	os.Setenv("YON_APPLICATION_JSON", string(raw))
}

func (setup JobsTestSetup) IdentityServer(member oidc.SigningRealm, installWide oidc.SigningRealm) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		asked := oidc.ServiceTokenRequest{}
		_ = json.NewDecoder(request.Body).Decode(&asked)
		token := strings.TrimPrefix(setup.Service(installWide.ForDomain(asked.DomainId)), "Bearer ")
		_ = json.NewEncoder(writer).Encode(oidc.ServiceTokenAnswer{AccessToken: token, ExpiresIn: 60})
	}))
	settings := member.Settings
	settings.ServiceTokens = server.URL + "/api/v1/identity/service-token"
	setup.Use(settings)
}

func (setup JobsTestSetup) Member(realm oidc.SigningRealm, memberId string) string {
	return setup.bearer(realm, oidc.TokenRequest{Subject: memberId, Name: "Ana Example", Username: "ana", Email: "ana@example.com", RealmRoles: []string{"mesail-member"}, Audience: "mesail-api", AuthorizedParty: "mesail-web", ExpiresAt: time.Now().Unix() + 60})
}

func (setup JobsTestSetup) Service(realm oidc.SigningRealm) string {
	return setup.bearer(realm, oidc.TokenRequest{Subject: "service-account", RealmRoles: []string{"mesail-service"}, Audience: "mesail-api", AuthorizedParty: "mesail-service", ExpiresAt: time.Now().Unix() + 60})
}

func (JobsTestSetup) bearer(realm oidc.SigningRealm, request oidc.TokenRequest) string {
	token, err := realm.Token(request)
	if err != nil {
		panic(err)
	}
	return "Bearer " + token
}

func (JobsTestSetup) Envelope(producer string, eventType string, domainId string, payload any) EventEnvelope {
	envelope, err := events.NewEnvelopeFactory(producer, time.Now, rand.Reader).New(eventType, domainId, payload)
	if err != nil {
		panic(err)
	}
	return EventEnvelope{EventId: envelope.EventID, Type: envelope.Type, DomainId: envelope.DomainId, Producer: envelope.Producer, OccurredAt: envelope.OccurredAt, Payload: envelope.Payload}
}

func (JobsTestSetup) Kit(envelope EventEnvelope) events.Envelope {
	return events.Envelope{EventID: envelope.EventId, Type: envelope.Type, DomainId: envelope.DomainId, Producer: envelope.Producer, OccurredAt: envelope.OccurredAt, Payload: envelope.Payload}
}

func (JobsTestSetup) Notification(domainId string, memberId string, messageId string) NotificationRecord {
	return NotificationRecord{
		DomainId:          domainId,
		MemberId:          memberId,
		Kind:              "mention",
		ConversationId:    "cnv_1",
		MessageId:         messageId,
		AuthorDisplayName: "Dan",
		DedupeKey:         messageId + "/" + memberId,
		CreatedAt:         (&TimestampService{}).Now(),
		PushState:         "pending",
		SummaryDueAt:      time.Now().Unix() - 1,
		SummaryState:      "waiting",
	}
}

func (JobsTestSetup) Documents() *DocumentsService {
	return &DocumentsService{}
}

func (setup JobsTestSetup) NotificationStore() *NotificationStoreService {
	return &NotificationStoreService{documents: setup.Documents(), names: &CollectionNamesService{}, states: &NotificationStatesService{}, timestamps: &TimestampService{}}
}

func (setup JobsTestSetup) PushStore() *PushSubscriptionStoreService {
	return &PushSubscriptionStoreService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup JobsTestSetup) SettingsStore() *MemberSettingsStoreService {
	return &MemberSettingsStoreService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup JobsTestSetup) Registry() *OutboxCollectionsService {
	return &OutboxCollectionsService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup JobsTestSetup) SummaryStore() *SummaryStoreService {
	return &SummaryStoreService{documents: setup.Documents(), names: &CollectionNamesService{}, registry: setup.Registry()}
}

func (setup JobsTestSetup) FanoutStore() *FanoutStoreService {
	return &FanoutStoreService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup JobsTestSetup) Domains() *DomainRegistryService {
	return &DomainRegistryService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup JobsTestSetup) OutboxStore() *OutboxStoreService {
	return &OutboxStoreService{documents: setup.Documents(), registry: setup.Registry()}
}

func (setup JobsTestSetup) Relay(pusher outbox.Pusher) *OutboxRelayService {
	return &OutboxRelayService{store: setup.OutboxStore(), settings: &NotifySettingsService{}, tokens: &ServiceTokenService{}, callers: &CallerService{}, pusher: pusher}
}

func (setup JobsTestSetup) StagedSummary(domainId string) SummaryRecord {
	settings, err := outbox.SettingsReader{}.FromApplicationJSON(`{"outbox":{"producer":"notify","subscribers":[{"name":"mail-out","url":"http://127.0.0.1:8110/api/v1/mail-out/events"}]}}`)
	if err != nil {
		panic(err)
	}
	payload := events.EmailSendRequested{MessageId: "sum_x", From: "notifications@" + domainId, Rcpt: "ana@" + domainId, Role: "to", To: []string{"ana@" + domainId}, Cc: []string{}, References: []string{}, AttachmentIds: []string{}, IdempotencyKey: "sum_x"}
	envelope := setup.Kit(setup.Envelope("notify", events.Types{}.EmailSendRequested(), domainId, payload))
	fields, err := outbox.NewOutbox(settings).Stage(outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{}}, envelope)
	if err != nil {
		panic(err)
	}
	return SummaryRecord{DomainId: domainId, MemberId: "member-1", BatchKey: setup.Name("batch"), Rcpt: "ana@" + domainId, Subject: "Unread", Body: "1 unread", NotificationIds: []string{"ntf_1"}, CreatedAt: (&TimestampService{}).Now(), PendingEvents: fields.PendingEvents, DeadEvents: fields.DeadEvents, OutboxPending: fields.OutboxPending, OutboxDead: fields.OutboxDead}
}

func (setup JobsTestSetup) Fanout() *FanoutService {
	queue := &FanoutQueueService{store: setup.FanoutStore(), timestamps: &TimestampService{}}
	writer := &FanoutWriterService{store: setup.NotificationStore(), states: &NotificationStatesService{}, timestamps: &TimestampService{}}
	return &FanoutService{queue: queue, recipients: &RecipientService{settings: setup.SettingsStore()}, writer: writer}
}
