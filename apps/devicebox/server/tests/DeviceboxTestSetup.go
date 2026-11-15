import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"mesail.dev/packages/apierrors"
	"mesail.dev/packages/collections"
	"mesail.dev/packages/devicebox"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @TestSetup
type DeviceboxTestSetup struct {
	Domain    string
	Directory devicebox.MemoryDirectory
	Pusher    *outbox.RecordingPusher
	Storage   devicebox.CountingStorage
	Clock     *time.Time
	Home      oidc.SigningRealm
	Away      oidc.SigningRealm
	Envelopes *EnvelopesController
	Jobs      *JobsController
	Outbox    *OutboxController
}

func (DeviceboxTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (DeviceboxTestSetup) World() *DeviceboxTestSetup {
	started := time.Now().UTC()
	world := &DeviceboxTestSetup{Directory: devicebox.NewMemoryDirectory(), Pusher: outbox.NewRecordingPusher(), Storage: devicebox.NewCountingStorage(DataSource{}), Clock: &started}
	world.Domain = "g6-" + world.RandomText() + ".test"
	world.Home, world.Away = world.SignedRealms()
	failures := &ErrorMappingService{}
	mailbox := &MailboxService{dataSource: DataSource{}, failures: failures, storage: world.Storage, pusher: world.Pusher, devices: world.Directory, clock: world}
	world.Envelopes = &EnvelopesController{mailbox: mailbox}
	world.Jobs = &JobsController{mailbox: mailbox}
	world.Outbox = &OutboxController{outbox: &OutboxService{dataSource: DataSource{}, failures: failures, pusher: world.Pusher}}
	return world
}

func (world *DeviceboxTestSetup) Now() time.Time {
	*world.Clock = world.Clock.Add(time.Millisecond)
	return *world.Clock
}

func (world *DeviceboxTestSetup) Later(duration time.Duration) {
	*world.Clock = world.Clock.Add(duration)
}

func (DeviceboxTestSetup) ApplicationJson(settings oidc.Settings) string {
	encoded, _ := json.Marshal(settings)
	return `{"yon":{"oidc":` + string(encoded) + `,"outbox":{"producer":"devicebox","subscribers":[` +
		`{"name":"realtime","url":"http://127.0.0.1:1/api/v1/realtime/events"},` +
		`{"name":"notify","url":"http://127.0.0.1:1/api/v1/notify/events"}]}}}`
}

func (world *DeviceboxTestSetup) SignedRealms() (oidc.SigningRealm, oidc.SigningRealm) {
	folder, err := os.MkdirTemp("", "devicebox-test-")
	world.Must(err)
	os.Setenv("TMPDIR", folder)
	realm, err := oidc.SigningRealm{}.StartedMesail()
	world.Must(err)
	world.Must(realm.Publish(time.Now()))
	os.Setenv("YON_APPLICATION_JSON", world.ApplicationJson(realm.Settings))
	return realm.ForDomain(world.Domain), realm.ForDomain("other-" + world.Domain)
}

func (world *DeviceboxTestSetup) Bearer(realm oidc.SigningRealm, personId string, realmRole string) string {
	request := oidc.TokenRequest{Subject: personId, Name: personId, Username: personId, Email: personId + "@" + realm.Domain, RealmRoles: []string{realmRole}, Audience: "mesail-api", AuthorizedParty: "mesail-web", ExpiresAt: time.Now().Unix() + 60}
	token, err := realm.Token(request)
	world.Must(err)
	return "Bearer " + token
}

func (world *DeviceboxTestSetup) Member(personId string) string {
	return world.Bearer(world.Home, personId, "mesail-member")
}

func (world *DeviceboxTestSetup) Stranger(personId string) string {
	return world.Bearer(world.Away, personId, "mesail-member")
}

func (world *DeviceboxTestSetup) Service() string {
	return world.Bearer(world.Home, "devicebox-job", "mesail-service")
}

func (world *DeviceboxTestSetup) Owns(personId string, deviceIds ...string) {
	world.Directory.Owns(world.Domain, personId, deviceIds...)
}

func (world *DeviceboxTestSetup) Request(sender string, to ...string) EnvelopePut {
	return EnvelopePut{EnvelopeId: world.EnvelopeId(), SenderDeviceId: sender, ToDeviceIds: to, Kind: "message", Ciphertext: world.Ciphertext(48)}
}

func (world *DeviceboxTestSetup) Put(personId string, sender string, to ...string) string {
	request := world.Request(sender, to...)
	_, err := world.Envelopes.PostEnvelope(world.Member(personId), request)
	world.Must(err)
	return request.EnvelopeId
}

func (world *DeviceboxTestSetup) Listed(personId string, device string) []string {
	page, err := world.Envelopes.GetEnvelopes(world.Member(personId), device, nil, nil)
	world.Must(err)
	return world.IdsOf(page)
}

func (world *DeviceboxTestSetup) IdsOf(page devicebox.EnvelopePage) []string {
	ids := []string{}
	for _, envelope := range page.Envelopes {
		ids = append(ids, envelope.EnvelopeId)
	}
	return ids
}

func (world *DeviceboxTestSetup) Text(value string) *string {
	return &value
}

func (world *DeviceboxTestSetup) Group(people int, devicesEach int) []string {
	devices := []string{}
	for index := 0; index < people*devicesEach; index++ {
		device := fmt.Sprintf("p%02d-d%d", index/devicesEach, index%devicesEach)
		world.Owns(fmt.Sprintf("person-%02d", index/devicesEach), device)
		devices = append(devices, device)
	}
	return devices
}

func (world *DeviceboxTestSetup) PutMany(count int, personId string, sender string, to ...string) {
	for index := 0; index < count; index++ {
		world.Put(personId, sender, to...)
	}
}

func (world *DeviceboxTestSetup) EnvelopeId() string {
	text := world.RandomText() + world.RandomText()
	return text[0:8] + "-" + text[8:12] + "-" + text[12:16] + "-" + text[16:20] + "-" + text[20:32]
}

func (world *DeviceboxTestSetup) Ciphertext(size int) string {
	random := make([]byte, size)
	rand.Read(random)
	return base64.RawURLEncoding.EncodeToString(random)
}

func (world *DeviceboxTestSetup) Close() {
	for _, domain := range []string{world.Domain, world.Away.Domain} {
		world.drop(domain)
	}
}

func (world *DeviceboxTestSetup) drop(domain string) {
	names := devicebox.Names{}
	source := DataSource{}
	window, _ := source.Scroll(names.Registry(), map[string]any{"name": names.Bodies(domain)}, "", 10)
	records := []devicebox.RegistryRecord{}
	window.Decode(&records)
	for _, record := range records {
		source.DeleteById(names.Registry(), record.Id)
	}
	for _, collection := range []string{names.Slips(domain), names.Bodies(domain), names.Counts(domain)} {
		collections.KnownCollections{}.Forget(collection)
		source.DropCollection(collection)
	}
}

func (world *DeviceboxTestSetup) RandomText() string {
	random := make([]byte, 8)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (world *DeviceboxTestSetup) Must(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v (cause: %v)", err, errors.Unwrap(err)))
	}
}

func (world *DeviceboxTestSetup) NumberOf(err error) int {
	var numbered apierrors.Numbered
	errors.As(err, &numbered)
	return world.numberOrZero(numbered)
}

func (world *DeviceboxTestSetup) numberOrZero(numbered apierrors.Numbered) int {
	if numbered == nil {
		return 0
	}
	return numbered.Number()
}
