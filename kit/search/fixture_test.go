package search

import (
	"encoding/json"
	"net/url"
	"strconv"
	"testing"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

const (
	domainA = "a.example"
	domainB = "b.example"
	tokenA  = "Bearer ana"
	tokenB  = "Bearer bob"
)

type fixture struct {
	t          *testing.T
	documents  *fakeDocuments
	reader     *fakeReader
	identifier *fakeIdentifier
	service    Service
	events     int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	documents := newFakeDocuments()
	reader := &fakeReader{readable: map[string][]string{}}
	identifier := &fakeIdentifier{callers: map[string]oidc.Caller{}}
	f := &fixture{t: t, documents: documents, reader: reader, identifier: identifier}
	f.service = NewService(documents, reader, identifier)
	f.member(tokenA, domainA, "mem_ana", "cnv_1", "cnv_2")
	f.member(tokenB, domainB, "mem_bob", "cnv_1", "cnv_2")
	return f
}

func (f *fixture) member(token string, domainId string, memberId string, conversations ...string) {
	f.identifier.callers[token] = oidc.Caller{DomainId: domainId, MemberId: memberId, Roles: []string{"Member"}}
	f.reader.readable[token] = conversations
}

func serviceCaller() oidc.Caller {
	return oidc.Caller{MemberId: "svc_messaging", Roles: []string{"Service"}}
}

func (f *fixture) envelope(eventType string, domainId string, payload any) events.Envelope {
	f.t.Helper()
	f.events++
	return envelopeWithId("evt_"+strconv.Itoa(f.events), eventType, domainId, payload)
}

func envelopeWithId(eventId string, eventType string, domainId string, payload any) events.Envelope {
	body, ok := payload.(json.RawMessage)
	if !ok {
		body, _ = json.Marshal(payload)
	}
	producer, _ := events.Catalog{}.ProducerOf(eventType)
	return events.Envelope{EventID: eventId, Type: eventType, DomainId: domainId, Producer: producer, OccurredAt: "2026-11-10T12:00:00Z", Payload: body}
}

func (f *fixture) receive(eventType string, domainId string, payload any) (events.Receipt, error) {
	f.t.Helper()
	return f.service.Receive(serviceCaller(), f.envelope(eventType, domainId, payload))
}

func (f *fixture) mustReceive(eventType string, domainId string, payload any) {
	f.t.Helper()
	receipt, err := f.receive(eventType, domainId, payload)
	if err != nil || receipt.Status != "applied" {
		f.t.Fatalf("receive %s: %+v %v", eventType, receipt, err)
	}
}

func (f *fixture) create(domainId string, message events.MessageCreated) {
	f.t.Helper()
	f.mustReceive(events.Types{}.MessageCreated(), domainId, message)
}

func (f *fixture) edit(domainId string, messageId string, body string) {
	f.t.Helper()
	f.mustReceive(events.Types{}.MessageEdited(), domainId, events.MessageEdited{MessageId: messageId, ConversationId: "cnv_1", Body: body, EditedAt: "2026-11-09T10:00:00Z"})
}

func (f *fixture) remove(domainId string, messageId string) {
	f.t.Helper()
	f.mustReceive(events.Types{}.MessageDeleted(), domainId, events.MessageDeleted{MessageId: messageId, ConversationId: "cnv_1", DeletedAt: "2026-11-09T11:00:00Z"})
}

func (f *fixture) search(token string, pairs ...string) Page {
	f.t.Helper()
	page, err := f.service.Search(token, query(pairs...))
	if err != nil {
		f.t.Fatalf("search %v: %v", pairs, err)
	}
	return page
}

func (f *fixture) searchIds(token string, pairs ...string) []string {
	f.t.Helper()
	return messageIds(f.search(token, pairs...))
}

func query(pairs ...string) url.Values {
	values := url.Values{}
	for index := 0; index+1 < len(pairs); index += 2 {
		values.Set(pairs[index], pairs[index+1])
	}
	return values
}

func messageIds(page Page) []string {
	ids := []string{}
	for _, hit := range page.Items {
		ids = append(ids, hit.MessageId)
	}
	return ids
}

func message(messageId string, conversationId string, body string, createdAt string) events.MessageCreated {
	return events.MessageCreated{
		MessageId:         messageId,
		ConversationId:    conversationId,
		AuthorMemberId:    "mem_ana",
		AuthorAddress:     "Ana@A.Example",
		AuthorDisplayName: "Ana Lima",
		Body:              body,
		CreatedAt:         createdAt,
	}
}

func sameIds(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
