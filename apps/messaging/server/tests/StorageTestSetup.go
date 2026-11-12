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
	"mesail.dev/packages/storage"
)

// @TestSetup
type StorageTestSetup struct {
	Domain      string
	Pusher      *outbox.RecordingPusher
	Names       storage.CollectionNames
	Storage     *StorageService
	Relay       *OutboxRelayService
	Settings    *SettingsService
	InstallWide oidc.SigningRealm
	Home        oidc.SigningRealm
}

func (StorageTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (StorageTestSetup) World() *StorageTestSetup {
	world := &StorageTestSetup{Pusher: outbox.NewRecordingPusher(), Settings: &SettingsService{}}
	world.Domain = "m3-" + world.RandomText() + ".test"
	world.Storage = &StorageService{dataSource: DataSource{}, settings: world.Settings, pusher: world.Pusher}
	world.Relay = &OutboxRelayService{storage: world.Storage, failures: &FailureService{}}
	world.InstallWide, world.Home = world.SignedRealms()
	return world
}

func (world *StorageTestSetup) SignedRealms() (oidc.SigningRealm, oidc.SigningRealm) {
	folder, err := os.MkdirTemp("", "messaging-test-")
	world.Must(err)
	os.Setenv("TMPDIR", folder)
	realm, err := oidc.SigningRealm{}.StartedMesail()
	world.Must(err)
	world.Must(realm.Publish(time.Now()))
	os.Setenv("YON_APPLICATION_JSON", world.ApplicationJson(realm.Settings, "http://127.0.0.1:1"))
	return realm.ForDomain(oidc.InstallWide{}.DomainId()), realm.ForDomain(world.Domain)
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

func (world *StorageTestSetup) Stores() core.Stores {
	return world.Storage.Storage().Stores()
}

func (world *StorageTestSetup) Conversations() core.ConversationStore {
	return world.Stores().Conversations
}

func (world *StorageTestSetup) Participants() core.ParticipantStore {
	return world.Stores().Participants
}

func (world *StorageTestSetup) Messages() core.MessageStore {
	return world.Stores().Messages
}

func (world *StorageTestSetup) Reactions() core.ReactionStore {
	return world.Stores().Reactions
}

func (world *StorageTestSetup) ThreadKeys() core.ThreadKeyStore {
	return world.Stores().ThreadKeys
}

func (world *StorageTestSetup) Mentions() core.MentionStore {
	return world.Stores().Mentions
}

func (world *StorageTestSetup) Members() core.MemberStore {
	return world.Stores().Members
}

func (world *StorageTestSetup) AttachmentUses() core.AttachmentUseStore {
	return world.Stores().AttachmentUses
}

func (world *StorageTestSetup) Seen() storage.SeenEvents {
	return world.Storage.Storage().Seen()
}

func (world *StorageTestSetup) Registry() storage.Registry {
	return world.Storage.Storage().Registry()
}

func (world *StorageTestSetup) OutboxStore() storage.OutboxStore {
	return world.Storage.Storage().Outbox().Store
}

func (world *StorageTestSetup) Core() *CoreService {
	return &CoreService{storage: world.Storage, settings: world.Settings}
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
	var failure *NotFound
	var storeFailure *core.NotFound
	return errors.As(err, &failure) || errors.As(err, &storeFailure)
}

func (world *StorageTestSetup) IsForbidden(err error) bool {
	var failure *Forbidden
	return errors.As(err, &failure)
}

func (world *StorageTestSetup) IsUnavailable(err error) bool {
	var failure *Unavailable
	var storeFailure *core.Unavailable
	return errors.As(err, &failure) || errors.As(err, &storeFailure)
}
