package channels

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

type fakeCallers map[string]oidc.Caller

func (callers fakeCallers) Caller(authorization string) (oidc.Caller, error) {
	caller, known := callers[authorization]
	if !known {
		return oidc.Caller{}, &Unauthenticated{Message: "unknown token"}
	}
	return caller, nil
}

type quietRelay struct{}

func (quietRelay) Drain(collection string, id string) {}

type storageDown struct{}

func (storageDown) Error() string {
	return "storage is down"
}

type world struct {
	t           *testing.T
	clock       time.Time
	documents   *MemoryDocuments
	directory   *MemoryDirectory
	pusher      *outbox.RecordingPusher
	desk        OutboxDesk
	workflow    Workflow
	general     ChannelDocument
	deadEventId string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{
		t:         t,
		clock:     time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
		documents: NewMemoryDocuments(),
		directory: NewMemoryDirectory(),
		pusher:    outbox.NewRecordingPusher(),
	}
	w.directory.Knows("ana", "bo", "cy", "dee")
	w.configure(settingsSnapshot())
	return w
}

func newQuietWorld(t *testing.T) *world {
	t.Helper()
	w := newWorld(t)
	w.workflow = NewWorkflow(w.ports(SettingsSource{}.FromSnapshot(settingsSnapshot()), quietRelay{}))
	return w
}

func settingsSnapshot() string {
	return `{"yon":{"outbox":{"producer":"channels","subscribers":[` +
		`{"name":"messaging","url":"http://127.0.0.1:1/m"},` +
		`{"name":"realtime","url":"http://127.0.0.1:1/r"},` +
		`{"name":"search","url":"http://127.0.0.1:1/s"}]},` +
		`"channels":{"directoryurl":"http://127.0.0.1:1","relayurl":"http://127.0.0.1:1/relay"}}}`
}

func (w *world) configure(snapshot string) {
	settings := SettingsSource{}.FromSnapshot(snapshot)
	w.desk = NewOutboxDesk(w.documents, w.pusher, settings, w.now)
	w.workflow = NewWorkflow(w.ports(settings, w.desk))
}

func (w *world) ports(settings Settings, relay Relay) Ports {
	return Ports{Documents: w.documents, Relay: relay, Directory: w.directory, Callers: testCallers(), Settings: settings, Now: w.now}
}

func testCallers() fakeCallers {
	return fakeCallers{
		"Bearer ana":    {DomainId: "example.com", MemberId: "ana", Roles: []string{"Member"}},
		"Bearer bo":     {DomainId: "example.com", MemberId: "bo", Roles: []string{"Member"}},
		"Bearer cy":     {DomainId: "example.com", MemberId: "cy", Roles: []string{"Member"}},
		"Bearer dee":    {DomainId: "example.com", MemberId: "dee", Roles: []string{"DomainAdmin"}},
		"Bearer eve":    {DomainId: "example.org", MemberId: "eve", Roles: []string{"Member"}},
		"Bearer broken": {DomainId: "not a domain", MemberId: "zed", Roles: []string{"Member"}},
	}
}

func (w *world) now() time.Time {
	return w.clock
}

func (w *world) create(token string, name string, visibility string) ChannelDocument {
	w.t.Helper()
	document, err := w.workflow.Create(token, CreateRequest{Name: name, Visibility: visibility})
	w.must(err)
	return document
}

func (w *world) update(token string, document ChannelDocument, request UpdateRequest) (ChannelDocument, error) {
	return w.workflow.Update(token, WireIds{}.ChannelId(document.Id), request)
}

func (w *world) join(token string, document ChannelDocument) error {
	return w.workflow.Join(token, WireIds{}.ChannelId(document.Id))
}

func (w *world) leave(token string, document ChannelDocument) error {
	return w.workflow.Leave(token, WireIds{}.ChannelId(document.Id))
}

func (w *world) invite(token string, document ChannelDocument, memberIds ...string) error {
	return w.workflow.Invite(token, WireIds{}.ChannelId(document.Id), memberIds)
}

func (w *world) must(err error) {
	w.t.Helper()
	if err != nil {
		w.t.Fatalf("unexpected error: %v", err)
	}
}

func (w *world) listed(token string) []string {
	w.t.Helper()
	documents, err := w.workflow.List(token)
	w.must(err)
	names := []string{}
	for _, document := range documents {
		names = append(names, document.Name)
	}
	return names
}

func (w *world) stored(domainId string, document ChannelDocument) ChannelDocument {
	w.t.Helper()
	found, _, err := NewStore(w.documents, quietRelay{}).FindChannel(domainId, document.Id)
	w.must(err)
	return found
}

func (w *world) membership(domainId string, document ChannelDocument, memberId string) MembershipDocument {
	w.t.Helper()
	found, _, err := NewStore(w.documents, quietRelay{}).FindMembership(domainId, document.Id, memberId)
	w.must(err)
	return found
}

func (w *world) types(subscriber string) []string {
	types := []string{}
	for _, envelope := range w.pusher.PushedTo(subscriber) {
		types = append(types, envelope.Type)
	}
	return types
}

func (w *world) pushed(subscriber string, eventType string) int {
	count := 0
	for _, found := range w.types(subscriber) {
		count = count + w.oneWhen(found == eventType)
	}
	return count
}

func (*world) oneWhen(match bool) int {
	if match {
		return 1
	}
	return 0
}

func (w *world) failStorageAt(call int) {
	for skipped := 1; skipped < call; skipped++ {
		w.documents.FailNext(nil)
	}
	w.documents.FailNext(storageDown{})
}

func everyStorageFailureSurfaces(t *testing.T, build func(t *testing.T) *world, operation func(w *world) error) {
	t.Helper()
	for call := 1; call < 200; call++ {
		w := build(t)
		w.failStorageAt(call)
		err := operation(w)
		if w.documents.Ensure("probe") != nil {
			wantTrue(t, call > 1, "the operation made no storage call")
			return
		}
		if !errors.Is(err, storageDown{}) {
			t.Fatalf("a storage failure at call %d gave %v", call, err)
		}
	}
	t.Fatal("the operation never ran out of storage calls")
}

func text(value string) *string {
	return &value
}

func flag(value bool) *bool {
	return &value
}

func wantError[T error](t *testing.T, err error) T {
	t.Helper()
	var target T
	if !errors.As(err, &target) {
		t.Fatalf("want %T, got %v", target, err)
	}
	return target
}

func wantField(t *testing.T, err error, field string) {
	t.Helper()
	invalid := wantError[*InvalidRequest](t, err)
	wantEqual(t, invalid.Field, field)
}

func wantNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func wantEqual(t *testing.T, got any, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func wantTrue(t *testing.T, condition bool, message string) {
	t.Helper()
	if !condition {
		t.Fatal(message)
	}
}

func wantContains(t *testing.T, value string, part string) {
	t.Helper()
	if !strings.Contains(value, part) {
		t.Fatalf("%q does not contain %q", value, part)
	}
}
