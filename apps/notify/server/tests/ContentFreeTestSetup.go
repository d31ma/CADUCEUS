import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/webpush"
)

// @TestSetup
type ContentFreeTestSetup struct{}

func (ContentFreeTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (setup ContentFreeTestSetup) Started() (string, oidc.SigningRealm, oidc.SigningRealm) {
	domainId := setup.Domain()
	member, installWide := setup.Realms(domainId)
	return domainId, member, installWide
}

func (ContentFreeTestSetup) Domain() string {
	return "d" + strings.ToLower(rand.Text())[:12] + ".example.com"
}

func (setup ContentFreeTestSetup) Realms(domainId string) (oidc.SigningRealm, oidc.SigningRealm) {
	folder, err := os.MkdirTemp("", "notify-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("TMPDIR", folder)
	realm, err := oidc.SigningRealm{}.StartedMesail()
	if err != nil {
		panic(err)
	}
	if err := realm.Publish(time.Now()); err != nil {
		panic(err)
	}
	setup.Use(realm.Settings)
	return realm.ForDomain(domainId), realm.ForDomain(oidc.InstallWide{}.DomainId())
}

func (ContentFreeTestSetup) Use(settings oidc.Settings) {
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
			"push": map[string]any{"enabled": true, "hosts": []string{"example.com"}},
			"vapid": map[string]any{
				"public":  map[string]any{"key": keys.PublicKey},
				"private": map[string]any{"key": keys.PrivateKey},
				"subject": "mailto:postmaster@example.com",
			},
			"presence": map[string]any{"url": ""},
			"summary":  map[string]any{"sender": "notifications"},
		},
	}}
	raw, err := json.Marshal(document)
	if err != nil {
		panic(err)
	}
	os.Setenv("YON_APPLICATION_JSON", string(raw))
}

func (ContentFreeTestSetup) ChangeNotify(change func(notify map[string]any)) {
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

func (setup ContentFreeTestSetup) WithPushTurnedOff() {
	setup.ChangeNotify(func(notify map[string]any) { notify["push"].(map[string]any)["enabled"] = false })
}

func (setup ContentFreeTestSetup) WithPresence(address string) {
	setup.ChangeNotify(func(notify map[string]any) { notify["presence"] = map[string]any{"url": address} })
}

func (setup ContentFreeTestSetup) IdentityServer(member oidc.SigningRealm, installWide oidc.SigningRealm) {
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

func (ContentFreeTestSetup) PresenceServer(states map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		answers := []Presence{}
		for _, memberId := range strings.Split(request.URL.Query().Get("memberIds"), ",") {
			answers = append(answers, Presence{MemberId: memberId, State: states[memberId]})
		}
		_ = json.NewEncoder(writer).Encode(answers)
	}))
}

func (setup ContentFreeTestSetup) Member(realm oidc.SigningRealm, memberId string) string {
	return setup.bearer(realm, oidc.TokenRequest{Subject: memberId, Name: "Ana Example", Username: "ana", Email: "ana@example.com", RealmRoles: []string{"mesail-member"}, Audience: "mesail-api", AuthorizedParty: "mesail-web", ExpiresAt: time.Now().Unix() + 60})
}

func (setup ContentFreeTestSetup) Service(realm oidc.SigningRealm) string {
	return setup.bearer(realm, oidc.TokenRequest{Subject: "service-account", RealmRoles: []string{"mesail-service"}, Audience: "mesail-api", AuthorizedParty: "mesail-service", ExpiresAt: time.Now().Unix() + 60})
}

func (ContentFreeTestSetup) bearer(realm oidc.SigningRealm, request oidc.TokenRequest) string {
	token, err := realm.Token(request)
	if err != nil {
		panic(err)
	}
	return "Bearer " + token
}

func (ContentFreeTestSetup) ClientFor(server *httptest.Server) *http.Client {
	transport := server.Client().Transport.(*http.Transport).Clone()
	address := server.Listener.Addr().String()
	transport.DialContext = func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func (ContentFreeTestSetup) PushStore() *PushSubscriptionStoreService {
	return &PushSubscriptionStoreService{documents: &DocumentsService{}, names: &CollectionNamesService{}}
}

func (ContentFreeTestSetup) SettingsStore() *MemberSettingsStoreService {
	return &MemberSettingsStoreService{documents: &DocumentsService{}, names: &CollectionNamesService{}}
}

func (setup ContentFreeTestSetup) PushServer(status int) (*httptest.Server, *sync.Map, chan map[string]any) {
	keys := &sync.Map{}
	inbox := make(chan map[string]any, 200)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		inbox <- setup.decrypted(keys, request.URL.Path, body)
		writer.WriteHeader(status)
	}))
	return server, keys, inbox
}

func (setup ContentFreeTestSetup) Subscribe(keys *sync.Map, domainId string, memberId string) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		panic(err)
	}
	path := "/send/" + strings.ToLower(rand.Text())
	keys.Store(path, []any{key, auth})
	record := PushSubscriptionRecord{
		DomainId:  domainId,
		MemberId:  memberId,
		Platform:  "web",
		Endpoint:  "https://push.example.com" + path,
		P256dh:    base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		Auth:      base64.RawURLEncoding.EncodeToString(auth),
		CreatedAt: (&TimestampService{}).Now(),
	}
	store := setup.PushStore()
	if err := store.Upsert(record); err != nil {
		panic(err)
	}
}

func (setup ContentFreeTestSetup) Person(domainId string, memberId string, state string) {
	store := setup.SettingsStore()
	profile := MemberProfilePatch{DisplayName: "Ana Secret-Name", PrimaryAddress: "ana@" + domainId, State: state}
	if err := store.PutProfile(domainId, memberId, profile); err != nil {
		panic(err)
	}
}

func (ContentFreeTestSetup) Received(inbox chan map[string]any) []map[string]any {
	received := []map[string]any{}
	for {
		select {
		case payload := <-inbox:
			received = append(received, payload)
		default:
			return received
		}
	}
}

func (ContentFreeTestSetup) Expected(title string, body string, tag string, kind string) map[string]any {
	return map[string]any{"title": title, "body": body, "tag": tag, "url": "/conversations", "notificationId": "", "kind": kind, "conversationId": "", "messageId": ""}
}

func (setup ContentFreeTestSetup) Pushes(client *http.Client) *ContentFreePushService {
	targets := &PushTargetsService{settings: &NotifySettingsService{}, client: client}
	presence := &PresenceService{settings: &NotifySettingsService{}}
	return &ContentFreePushService{targets: targets, presence: presence, sender: &PushSenderService{subscriptions: setup.PushStore()}}
}

func (setup ContentFreeTestSetup) Members() *MembershipService {
	return &MembershipService{settings: setup.SettingsStore()}
}

func (setup ContentFreeTestSetup) CallPushes(client *http.Client) *CallPushesService {
	return &CallPushesService{callers: &CallerService{}, members: setup.Members(), pushes: setup.Pushes(client)}
}

func (setup ContentFreeTestSetup) ChatPushes(client *http.Client) *ChatPushesService {
	return &ChatPushesService{members: setup.Members(), pushes: setup.Pushes(client)}
}

func (setup ContentFreeTestSetup) Events(client *http.Client) *EventsService {
	handlers := &EventHandlersService{handlers: &NotificationEventsService{}, chats: setup.ChatPushes(client)}
	seen := &SeenEventsService{documents: &DocumentsService{}, names: &CollectionNamesService{}, timestamps: &TimestampService{}}
	domains := &DomainRegistryService{documents: &DocumentsService{}, names: &CollectionNamesService{}}
	return &EventsService{callers: &CallerService{}, seen: seen, domains: domains, handlers: handlers}
}

func (ContentFreeTestSetup) Stored(domainId string, personId string, kind string) EventEnvelope {
	payload := events.EnvelopeStored{ToPersonId: personId, ToDeviceIds: []string{"dev_1", "dev_2"}, Kind: kind, EnvelopeId: "env_" + strings.ToLower(rand.Text())}
	envelope, err := events.NewEnvelopeFactory("devicebox", time.Now, rand.Reader).New(events.Types{}.EnvelopeStored(), domainId, payload)
	if err != nil {
		panic(err)
	}
	return EventEnvelope{EventId: envelope.EventID, Type: envelope.Type, DomainId: envelope.DomainId, Producer: envelope.Producer, OccurredAt: envelope.OccurredAt, Payload: envelope.Payload}
}

func (ContentFreeTestSetup) Kit(envelope EventEnvelope) events.Envelope {
	return events.Envelope{EventID: envelope.EventId, Type: envelope.Type, DomainId: envelope.DomainId, Producer: envelope.Producer, OccurredAt: envelope.OccurredAt, Payload: envelope.Payload}
}

func (ContentFreeTestSetup) decrypted(keys *sync.Map, path string, body []byte) map[string]any {
	stored, found := keys.Load(path)
	if !found {
		return map[string]any{"error": "unknown endpoint " + path}
	}
	pair := stored.([]any)
	privateKey, authSecret := pair[0].(*ecdh.PrivateKey), pair[1].([]byte)
	salt := body[:16]
	if binary.BigEndian.Uint32(body[16:20]) != 4096 {
		panic("record size")
	}
	keyLength := int(body[20])
	serverPublic := body[21 : 21+keyLength]
	record := body[21+keyLength:]
	serverKey, err := ecdh.P256().NewPublicKey(serverPublic)
	if err != nil {
		panic(err)
	}
	secret, err := privateKey.ECDH(serverKey)
	if err != nil {
		panic(err)
	}
	info := "WebPush: info\x00" + string(privateKey.PublicKey().Bytes()) + string(serverPublic)
	inputKey, err := hkdf.Key(sha256.New, secret, authSecret, info, 32)
	if err != nil {
		panic(err)
	}
	contentKey, err := hkdf.Key(sha256.New, inputKey, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		panic(err)
	}
	nonce, err := hkdf.Key(sha256.New, inputKey, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		panic(err)
	}
	block, err := aes.NewCipher(contentKey)
	if err != nil {
		panic(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	padded, err := aead.Open(nil, nonce, record, nil)
	if err != nil {
		panic(err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(bytes.TrimSuffix(padded, []byte{2}), &payload); err != nil {
		panic(err)
	}
	return payload
}

func (ContentFreeTestSetup) Text(received []map[string]any) string {
	raw, err := json.Marshal(received)
	if err != nil {
		panic(err)
	}
	return string(raw)
}
