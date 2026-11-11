import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @TestSetup
type StorageTestSetup struct {
	Domain    string
	Pusher    *outbox.RecordingPusher
	Documents *DocumentsService
	Names     *CollectionNamesService
	Registry  *OutboxCollectionsService
	Relay     *OutboxRelayService
	Tenant    *TenantCollectionsService
	Settings  *SettingsService
	Operators oidc.SigningRealm
	Home      oidc.SigningRealm
}

func (StorageTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (StorageTestSetup) World() *StorageTestSetup {
	world := &StorageTestSetup{Pusher: outbox.NewRecordingPusher(), Settings: &SettingsService{}, Names: &CollectionNamesService{}}
	world.Domain = "m3-" + world.RandomText() + ".test"
	world.Documents = &DocumentsService{dataSource: DataSource{}}
	world.Registry = &OutboxCollectionsService{documents: world.Documents, names: world.Names}
	world.Relay = &OutboxRelayService{store: &OutboxStoreService{documents: world.Documents, registry: world.Registry}, settings: world.Settings, pusher: world.Pusher}
	world.Tenant = &TenantCollectionsService{documents: world.Documents, registry: world.Registry, relay: world.Relay}
	world.Operators, world.Home = world.SignedRealms()
	return world
}

func (world *StorageTestSetup) SignedRealms() (oidc.SigningRealm, oidc.SigningRealm) {
	folder, err := os.MkdirTemp("", "messaging-test-")
	world.Must(err)
	os.Setenv("TMPDIR", folder)
	operators, err := oidc.SigningRealm{}.Started("mesail-operators")
	world.Must(err)
	home, err := oidc.SigningRealm{}.Started(world.Domain)
	world.Must(err)
	world.Must(operators.Publish(time.Now()))
	world.Must(home.Publish(time.Now()))
	os.Setenv("YON_APPLICATION_JSON", world.ApplicationJson(operators.Settings, "http://127.0.0.1:1"))
	return operators, home
}

func (world *StorageTestSetup) ApplicationJson(settings oidc.Settings, base string) string {
	encoded, _ := json.Marshal(settings)
	return `{"yon":{"oidc":` + string(encoded) + `,"outbox":{"producer":"messaging","subscribers":[` +
		`{"name":"realtime","url":"http://127.0.0.1:1/api/v1/realtime/events"},` +
		`{"name":"notify","url":"http://127.0.0.1:1/api/v1/notify/events"},` +
		`{"name":"search","url":"http://127.0.0.1:1/api/v1/search/events"},` +
		`{"name":"mail-out","url":"http://127.0.0.1:1/api/v1/mail-out/events"}]},` +
		`"messaging":{"directoryurl":"` + base + `","mailinurl":"` + base + `","relayurl":"` + base + `/api/v1/messaging/outbox/relay"}}}`
}

func (world *StorageTestSetup) Bearer(realm oidc.SigningRealm) string {
	request := oidc.TokenRequest{Subject: "service-account", RealmRoles: []string{"mesail-service"}, Audience: "mesail-api", AuthorizedParty: "mesail-service", ExpiresAt: time.Now().Unix() + 60}
	token, err := realm.Token(request)
	world.Must(err)
	return "Bearer " + token
}

func (world *StorageTestSetup) Conversations() *ConversationStoreService {
	return &ConversationStoreService{tenant: world.Tenant, names: world.Names}
}

func (world *StorageTestSetup) Participants() *ParticipantStoreService {
	return &ParticipantStoreService{tenant: world.Tenant, names: world.Names}
}

func (world *StorageTestSetup) Messages() *MessageStoreService {
	return &MessageStoreService{tenant: world.Tenant, names: world.Names}
}

func (world *StorageTestSetup) Reactions() *ReactionStoreService {
	return &ReactionStoreService{tenant: world.Tenant, names: world.Names}
}

func (world *StorageTestSetup) ThreadKeys() *ThreadKeyStoreService {
	return &ThreadKeyStoreService{tenant: world.Tenant, names: world.Names}
}

func (world *StorageTestSetup) Mentions() *MentionStoreService {
	return &MentionStoreService{tenant: world.Tenant, names: world.Names}
}

func (world *StorageTestSetup) Members() *MemberStoreService {
	return &MemberStoreService{tenant: world.Tenant, names: world.Names}
}

func (world *StorageTestSetup) AttachmentUses() *AttachmentUseStoreService {
	return &AttachmentUseStoreService{tenant: world.Tenant, names: world.Names}
}

func (world *StorageTestSetup) Seen() *SeenEventsService {
	return &SeenEventsService{tenant: world.Tenant, names: world.Names}
}

func (world *StorageTestSetup) Core() *CoreService {
	chat := &ChatStoresService{conversations: world.Conversations(), participants: world.Participants(), messages: world.Messages(), reactions: world.Reactions()}
	lookup := &LookupStoresService{threadKeys: world.ThreadKeys(), mentions: world.Mentions(), members: world.Members(), attachmentUses: world.AttachmentUses()}
	return &CoreService{chatStores: chat, lookupStores: lookup, settings: world.Settings}
}

func (world *StorageTestSetup) Staged(fields core.OutboxFields, eventType string, payload any) core.OutboxFields {
	settings, err := world.Settings.Outbox()
	world.Must(err)
	staged, err := core.NewStager(settings, time.Now, rand.Reader).Stage(fields, eventType, world.Domain, payload)
	world.Must(err)
	return staged
}

func (world *StorageTestSetup) Pushed(subscriber string, eventType string) int {
	count := 0
	for _, envelope := range world.Pusher.PushedTo(subscriber) {
		count += world.countIf(envelope.Type == eventType && envelope.DomainId == world.Domain)
	}
	return count
}

func (world *StorageTestSetup) countIf(matches bool) int {
	if matches {
		return 1
	}
	return 0
}

func (world *StorageTestSetup) RandomText() string {
	random := make([]byte, 8)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (world *StorageTestSetup) Must(err error) {
	if err != nil {
		panic(err)
	}
}

func (world *StorageTestSetup) IsNotFound(err error) bool {
	var failure *core.NotFound
	return errors.As(err, &failure)
}

func (world *StorageTestSetup) IsForbidden(err error) bool {
	var failure *core.Forbidden
	return errors.As(err, &failure)
}

func (world *StorageTestSetup) IsUnavailable(err error) bool {
	var failure *core.Unavailable
	return errors.As(err, &failure)
}
