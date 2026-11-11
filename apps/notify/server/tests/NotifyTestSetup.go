import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/webpush"
)

// @TestSetup
type NotifyTestSetup struct{}

func (NotifyTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (NotifyTestSetup) Domain() string {
	return "d" + strings.ToLower(rand.Text())[:12] + ".example.com"
}

func (NotifyTestSetup) Name(base string) string {
	return base + "-" + strings.ToLower(rand.Text())[:12]
}

func (setup NotifyTestSetup) Realms(domainId string) (oidc.SigningRealm, oidc.SigningRealm) {
	folder, err := os.MkdirTemp("", "notify-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("TMPDIR", folder)
	member := setup.started(domainId)
	operators := setup.started("mesail-operators")
	setup.Use(member.Settings)
	return member, operators
}

func (NotifyTestSetup) started(name string) oidc.SigningRealm {
	realm, err := oidc.SigningRealm{}.Started(name)
	if err != nil {
		panic(err)
	}
	if err := realm.Publish(time.Now()); err != nil {
		panic(err)
	}
	return realm
}

func (NotifyTestSetup) Use(settings oidc.Settings) {
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
			"push": map[string]any{"hosts": webpush.EndpointPolicy{}.DefaultHosts()},
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

func (setup NotifyTestSetup) Keycloak(member oidc.SigningRealm, operators oidc.SigningRealm) {
	token := strings.TrimPrefix(setup.Service(operators), "Bearer ")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": token, "expires_in": 60})
	}))
	settings := member.Settings
	settings.Keycloak = server.URL + "/realms/{realm}/protocol/openid-connect"
	setup.Use(settings)
}

func (setup NotifyTestSetup) Member(realm oidc.SigningRealm, memberId string) string {
	return setup.bearer(realm, oidc.TokenRequest{Subject: memberId, Name: "Ana Example", Email: "ana@example.com", RealmRoles: []string{"mesail-member"}, Audience: "mesail-api", AuthorizedParty: "mesail-identity", ExpiresAt: time.Now().Unix() + 60})
}

func (setup NotifyTestSetup) Service(realm oidc.SigningRealm) string {
	return setup.bearer(realm, oidc.TokenRequest{Subject: "service-account", RealmRoles: []string{"mesail-service"}, Audience: "mesail-api", AuthorizedParty: "mesail-service", ExpiresAt: time.Now().Unix() + 60})
}

func (NotifyTestSetup) bearer(realm oidc.SigningRealm, request oidc.TokenRequest) string {
	token, err := realm.Token(request)
	if err != nil {
		panic(err)
	}
	return "Bearer " + token
}

func (NotifyTestSetup) Envelope(producer string, eventType string, domainId string, payload any) EventEnvelope {
	envelope, err := events.NewEnvelopeFactory(producer, time.Now, rand.Reader).New(eventType, domainId, payload)
	if err != nil {
		panic(err)
	}
	return EventEnvelope{EventId: envelope.EventID, Type: envelope.Type, DomainId: envelope.DomainId, Producer: envelope.Producer, OccurredAt: envelope.OccurredAt, Payload: envelope.Payload}
}

func (NotifyTestSetup) Kit(envelope EventEnvelope) events.Envelope {
	return events.Envelope{EventID: envelope.EventId, Type: envelope.Type, DomainId: envelope.DomainId, Producer: envelope.Producer, OccurredAt: envelope.OccurredAt, Payload: envelope.Payload}
}

func (NotifyTestSetup) Notification(domainId string, memberId string, messageId string) NotificationRecord {
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

func (NotifyTestSetup) Documents() *DocumentsService {
	return &DocumentsService{}
}

func (setup NotifyTestSetup) NotificationStore() *NotificationStoreService {
	return &NotificationStoreService{documents: setup.Documents(), names: &CollectionNamesService{}, states: &NotificationStatesService{}, timestamps: &TimestampService{}}
}

func (setup NotifyTestSetup) PushStore() *PushSubscriptionStoreService {
	return &PushSubscriptionStoreService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup NotifyTestSetup) SettingsStore() *MemberSettingsStoreService {
	return &MemberSettingsStoreService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup NotifyTestSetup) ThreadStore() *ThreadStoreService {
	return &ThreadStoreService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup NotifyTestSetup) Registry() *OutboxCollectionsService {
	return &OutboxCollectionsService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup NotifyTestSetup) SummaryStore() *SummaryStoreService {
	return &SummaryStoreService{documents: setup.Documents(), names: &CollectionNamesService{}, registry: setup.Registry(), outboxStates: &OutboxStatesService{}}
}

func (setup NotifyTestSetup) FanoutStore() *FanoutStoreService {
	return &FanoutStoreService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup NotifyTestSetup) Domains() *DomainRegistryService {
	return &DomainRegistryService{documents: setup.Documents(), names: &CollectionNamesService{}}
}

func (setup NotifyTestSetup) Seen() *SeenEventsService {
	return &SeenEventsService{documents: setup.Documents(), names: &CollectionNamesService{}, timestamps: &TimestampService{}}
}

func (setup NotifyTestSetup) OutboxStore() *OutboxStoreService {
	return &OutboxStoreService{documents: setup.Documents(), registry: setup.Registry(), outboxStates: &OutboxStatesService{}}
}

func (setup NotifyTestSetup) Relay(pusher outbox.Pusher) *OutboxRelayService {
	return &OutboxRelayService{store: setup.OutboxStore(), settings: &NotifySettingsService{}, tokens: &OperatorTokenService{}, callers: &CallerService{}, pusher: pusher}
}

func (setup NotifyTestSetup) Notifications() *NotificationsService {
	return &NotificationsService{callers: &CallerService{}, store: setup.NotificationStore(), timestamps: &TimestampService{}}
}

func (setup NotifyTestSetup) Subscriptions() *PushSubscriptionsService {
	return &PushSubscriptionsService{callers: &CallerService{}, store: setup.PushStore(), settings: &NotifySettingsService{}, timestamps: &TimestampService{}}
}

func (NotifyTestSetup) PushKeys() *PushKeyService {
	return &PushKeyService{callers: &CallerService{}, settings: &NotifySettingsService{}}
}

func (setup NotifyTestSetup) Preferences() *PreferencesService {
	return &PreferencesService{callers: &CallerService{}, store: setup.SettingsStore()}
}

func (setup NotifyTestSetup) EventActions() *NotificationEventsService {
	return &NotificationEventsService{settings: setup.SettingsStore(), notifications: setup.NotificationStore(), timestamps: &TimestampService{}, created: &MessageCreatedService{}}
}

func (setup NotifyTestSetup) Handlers() *EventHandlersService {
	return &EventHandlersService{handlers: setup.EventActions()}
}

func (setup NotifyTestSetup) Events() *EventsService {
	return &EventsService{callers: &CallerService{}, seen: setup.Seen(), domains: setup.Domains(), handlers: setup.Handlers()}
}

func (setup NotifyTestSetup) StagedSummary(domainId string) SummaryRecord {
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
