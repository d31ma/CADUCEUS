package devicebox

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

type fakeCallers struct{}

func (fakeCallers) Member(authorization string) (oidc.Caller, error) {
	return fakeCallers{}.parse(authorization, true)
}

func (fakeCallers) Service(authorization string) (oidc.Caller, error) {
	return fakeCallers{}.parse(authorization, false)
}

func (fakeCallers) parse(authorization string, member bool) (oidc.Caller, error) {
	var domain, person, role string
	if _, err := fmt.Sscanf(authorization, "%s %s %s", &domain, &person, &role); err != nil {
		return oidc.Caller{}, Unauthenticated{}.TokenRefused()
	}
	if member && role != "Member" {
		return oidc.Caller{}, Forbidden{}.NotADomainMember()
	}
	return oidc.Caller{DomainId: domain, MemberId: person, Roles: []string{role}}, nil
}

type world struct {
	t         *testing.T
	storage   CountingStorage
	fylo      *fyloStorage
	directory MemoryDirectory
	pusher    *outbox.RecordingPusher
	clock     time.Time
	domain    string
	mailbox   Mailbox
	expiry    Expiry
	desk      OutboxDesk
}

func newWorld(t *testing.T) *world {
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"outbox":{"producer":"devicebox","subscribers":[`+
		`{"name":"realtime","url":"http://127.0.0.1:1/api/v1/realtime/events"},`+
		`{"name":"notify","url":"http://127.0.0.1:1/api/v1/notify/events"}]}}}`)
	fylo := newFyloStorage(t)
	w := &world{t: t, fylo: fylo, storage: NewCountingStorage(fylo), directory: NewMemoryDirectory(), pusher: outbox.NewRecordingPusher(), clock: time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC), domain: "example.com"}
	assembly := Assembly{Source: w.storage, Pusher: w.pusher, Directory: w.directory, Now: w.now}
	w.mailbox = assembly.Mailbox()
	w.mailbox.callers = fakeCallers{}
	w.expiry = assembly.Expiry()
	w.expiry.callers = fakeCallers{}
	w.desk = assembly.Desk()
	return w
}

func (w *world) now() time.Time {
	w.clock = w.clock.Add(time.Millisecond)
	return w.clock
}

func (w *world) member(person string) string {
	return w.domain + " " + person + " Member"
}

func (w *world) service() string {
	return w.domain + " job Service"
}

func (w *world) envelopeId() string {
	random := make([]byte, 16)
	rand.Read(random)
	text := hex.EncodeToString(random)
	return text[0:8] + "-" + text[8:12] + "-" + text[12:16] + "-" + text[16:20] + "-" + text[20:32]
}

func (w *world) ciphertext(size int) string {
	random := make([]byte, size)
	rand.Read(random)
	return base64.RawURLEncoding.EncodeToString(random)
}

func (w *world) put(person string, sender string, to ...string) (EnvelopeReceipt, string, error) {
	id := w.envelopeId()
	receipt, err := w.mailbox.Put(w.member(person), PutRequest{EnvelopeId: id, SenderDeviceId: sender, ToDeviceIds: to, Kind: "message", Ciphertext: w.ciphertext(64)})
	return receipt, id, err
}

func (w *world) mustPut(person string, sender string, to ...string) string {
	w.t.Helper()
	_, id, err := w.put(person, sender, to...)
	if err != nil {
		w.t.Fatalf("put failed: %v", err)
	}
	return id
}

func (w *world) list(person string, device string) EnvelopePage {
	w.t.Helper()
	page, err := w.mailbox.List(w.member(person), device, "", "")
	if err != nil {
		w.t.Fatalf("list failed: %v", err)
	}
	return page
}

func (w *world) ids(page EnvelopePage) []string {
	ids := []string{}
	for _, envelope := range page.Envelopes {
		ids = append(ids, envelope.EnvelopeId)
	}
	return ids
}

func (w *world) stored(subscriber string) []events.EnvelopeStored {
	payloads := []events.EnvelopeStored{}
	for _, envelope := range w.pusher.PushedTo(subscriber) {
		payload := events.EnvelopeStored{}
		events.PayloadReader{}.Read(envelope, &payload)
		payloads = append(payloads, payload)
	}
	return payloads
}

func (w *world) countRows(collection string, field string, value string) int {
	rows := []map[string]any{}
	statement, _ := SqlText{}.firstWhere(collection, field, value)
	statement.Sql = statement.Sql[:len(statement.Sql)-len(" LIMIT 1")] + " LIMIT 100"
	NewDocuments(w.fylo).Rows(statement, &rows)
	return len(rows)
}

func numberOf(err error) int {
	type numbered interface{ Number() int }
	value, ok := err.(numbered)
	if !ok {
		return 0
	}
	return value.Number()
}
