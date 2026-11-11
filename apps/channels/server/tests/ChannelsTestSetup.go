import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"mesail.dev/packages/channels"
	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @TestSetup
type ChannelsTestSetup struct {
	Domain     string
	Directory  *channels.MemoryDirectory
	Pusher     *outbox.RecordingPusher
	Home       oidc.SigningRealm
	Away       oidc.SigningRealm
	Controller *ChannelsController
	Outbox     *OutboxController
}

func (ChannelsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (ChannelsTestSetup) World() *ChannelsTestSetup {
	world := &ChannelsTestSetup{Directory: channels.NewMemoryDirectory(), Pusher: outbox.NewRecordingPusher()}
	world.Domain = "c2-" + world.RandomText() + ".test"
	world.Home, world.Away = world.SignedRealms()
	world.Controller = &ChannelsController{dataSource: DataSource{}, pusher: world.Pusher, lookup: world.Directory}
	world.Outbox = &OutboxController{dataSource: DataSource{}, pusher: world.Pusher}
	return world
}

func (ChannelsTestSetup) ApplicationJson(settings oidc.Settings) string {
	encoded, _ := json.Marshal(settings)
	return `{"yon":{"oidc":` + string(encoded) + `,"outbox":{"producer":"channels","subscribers":[` +
		`{"name":"messaging","url":"http://127.0.0.1:1/api/v1/messaging/events"},` +
		`{"name":"realtime","url":"http://127.0.0.1:1/api/v1/realtime/events"},` +
		`{"name":"search","url":"http://127.0.0.1:1/api/v1/search/events"}]},` +
		`"channels":{"directoryurl":"http://127.0.0.1:1"}}}`
}

func (world *ChannelsTestSetup) SignedRealms() (oidc.SigningRealm, oidc.SigningRealm) {
	folder, err := os.MkdirTemp("", "channels-test-")
	world.Must(err)
	os.Setenv("TMPDIR", folder)
	home, err := oidc.SigningRealm{}.Started(world.Domain)
	world.Must(err)
	away, err := oidc.SigningRealm{}.Started("other-" + world.Domain)
	world.Must(err)
	world.Must(home.Publish(time.Now()))
	world.Must(away.Publish(time.Now()))
	os.Setenv("YON_APPLICATION_JSON", world.ApplicationJson(home.Settings))
	return home, away
}

func (world *ChannelsTestSetup) Bearer(realm oidc.SigningRealm, memberId string, realmRole string) string {
	request := oidc.TokenRequest{Subject: memberId, Name: memberId, Email: memberId + "@" + realm.Name, RealmRoles: []string{realmRole}, Audience: "mesail-api", AuthorizedParty: "mesail-identity", ExpiresAt: time.Now().Unix() + 60}
	token, err := realm.Token(request)
	world.Must(err)
	return "Bearer " + token
}

func (world *ChannelsTestSetup) Member(memberId string) string {
	return world.Bearer(world.Home, memberId, "mesail-member")
}

func (world *ChannelsTestSetup) Admin(memberId string) string {
	return world.Bearer(world.Home, memberId, "mesail-admin")
}

func (world *ChannelsTestSetup) Stranger(memberId string) string {
	return world.Bearer(world.Away, memberId, "mesail-member")
}

func (world *ChannelsTestSetup) Create(authorization string, name string, visibility string) Channel {
	channel, err := world.Controller.PostChannel(authorization, CreateChannelRequest{Name: name, Topic: "a topic", Visibility: visibility})
	world.Must(err)
	return channel
}

func (world *ChannelsTestSetup) ListedNames(authorization string) []string {
	listed, err := world.Controller.GetChannels(authorization)
	world.Must(err)
	names := []string{}
	for _, channel := range listed {
		names = append(names, channel.Name)
	}
	return names
}

func (world *ChannelsTestSetup) Published(subscriber string, eventType string, part string) []events.Envelope {
	matching := []events.Envelope{}
	for _, envelope := range world.Pusher.PushedTo(subscriber) {
		encoded, _ := json.Marshal(envelope)
		matching = append(matching, world.ifMatching(envelope, string(encoded), eventType, part)...)
	}
	return matching
}

func (world *ChannelsTestSetup) ifMatching(envelope events.Envelope, encoded string, eventType string, part string) []events.Envelope {
	if envelope.Type == eventType && envelope.DomainId == world.Domain && strings.Contains(encoded, part) {
		return []events.Envelope{envelope}
	}
	return []events.Envelope{}
}

func (world *ChannelsTestSetup) Close() {
	domains := []string{world.Domain, world.Away.Name}
	for _, domain := range domains {
		world.drop(channels.Names{}.Channels(domain))
		world.drop(channels.Names{}.Memberships(domain))
	}
}

func (world *ChannelsTestSetup) drop(collection string) {
	source := DataSource{}
	window, _ := source.Scroll(channels.Names{}.Registry(), map[string]any{"name": collection}, "", 100)
	records := []channels.CollectionRecord{}
	window.Decode(&records)
	for _, record := range records {
		source.DeleteById(channels.Names{}.Registry(), record.Id)
	}
	source.DropCollection(collection)
}

func (world *ChannelsTestSetup) RandomText() string {
	random := make([]byte, 8)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (world *ChannelsTestSetup) Must(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v (cause: %v)", err, errors.Unwrap(err)))
	}
}

func (world *ChannelsTestSetup) IsNotFound(err error) bool {
	var failure *NotFound
	return errors.As(err, &failure)
}

func (world *ChannelsTestSetup) IsForbidden(err error) bool {
	var failure *Forbidden
	return errors.As(err, &failure)
}

func (world *ChannelsTestSetup) IsConflict(err error) bool {
	var failure *Conflict
	return errors.As(err, &failure)
}

func (world *ChannelsTestSetup) IsDuplicate(err error) bool {
	var failure *Duplicate
	return errors.As(err, &failure)
}

func (world *ChannelsTestSetup) IsUnauthenticated(err error) bool {
	var failure *Unauthenticated
	return errors.As(err, &failure)
}

func (world *ChannelsTestSetup) InvalidField(err error) string {
	var failure *InvalidRequest
	errors.As(err, &failure)
	return world.FieldOf(failure)
}

func (world *ChannelsTestSetup) FieldOf(failure *InvalidRequest) string {
	if failure == nil {
		return ""
	}
	return failure.Field
}
