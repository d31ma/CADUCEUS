package search

import (
	"encoding/json"
	"errors"
	"testing"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

func TestEditReplacesTheSearchableText(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_1", "cnv_1", "alpha words", "2026-11-05T10:00:00Z"))
	f.edit(domainA, "msg_1", "bravo words")
	sameIds(t, f.searchIds(tokenA, "q", "alpha"))
	page := f.search(tokenA, "q", "bravo")
	sameIds(t, messageIds(page), "msg_1")
	if page.Items[0].Snippet != "bravo words" {
		t.Fatalf("got %+v", page.Items[0])
	}
}

func TestEditKeepsTheOtherFieldsSearchable(t *testing.T) {
	f := newFixture(t)
	created := message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z")
	created.Subject = "harvest"
	f.create(domainA, created)
	f.edit(domainA, "msg_1", "bravo")
	sameIds(t, f.searchIds(tokenA, "q", "harvest"), "msg_1")
	sameIds(t, f.searchIds(tokenA, "q", "lima"), "msg_1")
}

func TestDeleteHidesTheMessage(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	f.create(domainA, message("msg_2", "cnv_1", "alpha", "2026-11-04T10:00:00Z"))
	f.remove(domainA, "msg_1")
	sameIds(t, f.searchIds(tokenA, "q", "alpha"), "msg_2")
	sameIds(t, f.searchIds(tokenA), "msg_2")
}

func TestDeleteBeforeCreateKeepsTheMessageHidden(t *testing.T) {
	f := newFixture(t)
	f.remove(domainA, "msg_1")
	f.create(domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	sameIds(t, f.searchIds(tokenA, "q", "alpha"))
}

func TestEditAfterDeleteKeepsTheMessageHidden(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	f.remove(domainA, "msg_1")
	f.edit(domainA, "msg_1", "bravo")
	sameIds(t, f.searchIds(tokenA, "q", "bravo"))
}

func TestDeleteInOneDomainLeavesTheOtherDomain(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	f.create(domainB, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	f.remove(domainB, "msg_1")
	sameIds(t, f.searchIds(tokenA, "q", "alpha"), "msg_1")
	sameIds(t, f.searchIds(tokenB, "q", "alpha"))
}

func TestEditOfAnUnknownMessageDoesNothing(t *testing.T) {
	f := newFixture(t)
	f.edit(domainA, "msg_unknown", "bravo")
	sameIds(t, f.searchIds(tokenA))
}

func TestTheSameEventTwiceIsAppliedOnce(t *testing.T) {
	f := newFixture(t)
	envelope := f.envelope(events.Types{}.MessageCreated(), domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	first, err := f.service.Receive(serviceCaller(), envelope)
	if err != nil || first.Status != "applied" || first.EventID != envelope.EventID {
		t.Fatalf("got %+v %v", first, err)
	}
	second, err := f.service.Receive(serviceCaller(), envelope)
	if err != nil || second.Status != "duplicate" || second.EventID != envelope.EventID {
		t.Fatalf("got %+v %v", second, err)
	}
	sameIds(t, f.searchIds(tokenA), "msg_1")
}

func TestTheSameEventIdInAnotherDomainIsApplied(t *testing.T) {
	f := newFixture(t)
	created := message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z")
	_, err := f.service.Receive(serviceCaller(), envelopeWithId("evt_same", "message.created", domainA, created))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := f.service.Receive(serviceCaller(), envelopeWithId("evt_same", "message.created", domainB, created))
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("got %+v %v", receipt, err)
	}
	sameIds(t, f.searchIds(tokenB), "msg_1")
}

func TestASecondCreateOfTheSameMessageDoesNotDuplicateHits(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	f.create(domainA, message("msg_1", "cnv_1", "alpha again", "2026-11-05T10:00:00Z"))
	sameIds(t, f.searchIds(tokenA, "q", "alpha"), "msg_1")
}

func TestAnEventTheIndexerIgnoresIsStillApplied(t *testing.T) {
	f := newFixture(t)
	f.mustReceive(events.Types{}.MemberUpserted(), domainA, events.MemberUpserted{MemberId: "mem_ana", DisplayName: "Ana"})
	sameIds(t, f.searchIds(tokenA))
}

func TestReceiveRefusesBadPayloads(t *testing.T) {
	types := events.Types{}
	cases := []struct {
		name      string
		eventType string
		payload   any
		field     string
	}{
		{"created shape", types.MessageCreated(), json.RawMessage(`[1]`), "payload"},
		{"created unknown field", types.MessageCreated(), json.RawMessage(`{"messageId":"msg_1","bogus":1}`), "payload"},
		{"created data after the value", types.MessageCreated(), json.RawMessage(`{"messageId":"msg_1"} {}`), "payload"},
		{"edited shape", types.MessageEdited(), json.RawMessage(`"text"`), "payload"},
		{"edited unknown field", types.MessageEdited(), json.RawMessage(`{"bogus":true}`), "payload"},
		{"deleted shape", types.MessageDeleted(), json.RawMessage(`7`), "payload"},
		{"deleted unknown field", types.MessageDeleted(), json.RawMessage(`{"bogus":true}`), "payload"},
		{"created bad time", types.MessageCreated(), message("msg_1", "cnv_1", "alpha", "yesterday"), "createdAt"},
		{"created no time", types.MessageCreated(), message("msg_1", "cnv_1", "alpha", ""), "createdAt"},
		{"created no message id", types.MessageCreated(), message("", "cnv_1", "alpha", "2026-11-05T10:00:00Z"), "payload"},
		{"created no conversation id", types.MessageCreated(), message("msg_1", "", "alpha", "2026-11-05T10:00:00Z"), "payload"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.receive(test.eventType, domainA, test.payload)
			refusal := InvalidRequest{}
			if !errors.As(err, &refusal) || refusal.Field != test.field {
				t.Fatalf("got %#v", err)
			}
			sameIds(t, f.searchIds(tokenA))
		})
	}
}

func TestReceiveRefusesABadEditTime(t *testing.T) {
	f := newFixture(t)
	f.create(domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	_, err := f.receive(events.Types{}.MessageEdited(), domainA, events.MessageEdited{MessageId: "msg_1", Body: "bravo", EditedAt: "later"})
	refusal := InvalidRequest{}
	if !errors.As(err, &refusal) || refusal.Field != "editedAt" {
		t.Fatalf("got %#v", err)
	}
	sameIds(t, f.searchIds(tokenA, "q", "alpha"), "msg_1")
}

func TestReceiveRefusesACallerWithoutTheServiceRole(t *testing.T) {
	f := newFixture(t)
	envelope := f.envelope(events.Types{}.MessageCreated(), domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	_, err := f.service.Receive(oidc.Caller{DomainId: domainA, Roles: []string{"Member"}}, envelope)
	refusal := Forbidden{}
	if !errors.As(err, &refusal) || refusal.Message != (events.NotService{}).Error() {
		t.Fatalf("got %#v", err)
	}
	sameIds(t, f.searchIds(tokenA))
}

func TestReceiveRefusesAnEventTypeSearchDoesNotConsume(t *testing.T) {
	f := newFixture(t)
	_, err := f.receive(events.Types{}.EmailReceived(), domainA, events.EmailReceived{InboundId: "in_1"})
	refusal := InvalidRequest{}
	if !errors.As(err, &refusal) || refusal.Field != "envelope" {
		t.Fatalf("got %#v", err)
	}
	if refusal.Message != (events.NotSubscribed{Service: "search", Type: "email.received"}).Error() {
		t.Fatalf("got %q", refusal.Message)
	}
}

func TestReceiveRefusesAnInvalidEnvelope(t *testing.T) {
	cases := map[string]func(*events.Envelope){
		"no event id":    func(envelope *events.Envelope) { envelope.EventID = "" },
		"bad domain":     func(envelope *events.Envelope) { envelope.DomainId = "Not A Domain" },
		"wrong producer": func(envelope *events.Envelope) { envelope.Producer = "channels" },
		"unknown type":   func(envelope *events.Envelope) { envelope.Type = "message.exploded" },
		"bad time":       func(envelope *events.Envelope) { envelope.OccurredAt = "soon" },
		"no payload":     func(envelope *events.Envelope) { envelope.Payload = nil },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			envelope := f.envelope(events.Types{}.MessageCreated(), domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
			change(&envelope)
			_, err := f.service.Receive(serviceCaller(), envelope)
			refusal := InvalidRequest{}
			if !errors.As(err, &refusal) || refusal.Field != "envelope" {
				t.Fatalf("got %#v", err)
			}
		})
	}
}

func TestReceiveIsUnavailableWhenSeenEventsCannotBeRead(t *testing.T) {
	f := newFixture(t)
	f.documents.failNext("CollectionExists")
	_, err := f.receive(events.Types{}.MessageCreated(), domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	if !errors.As(err, &Unavailable{}) || !errors.Is(err, errFakeStore) {
		t.Fatalf("got %#v", err)
	}
}

func TestReceiveIsUnavailableWhenTheEventCannotBeRemembered(t *testing.T) {
	f := newFixture(t)
	f.mustReceive(events.Types{}.MemberUpserted(), domainA, events.MemberUpserted{MemberId: "mem_ana"})
	f.documents.failNext("Insert")
	_, err := f.receive(events.Types{}.MemberUpserted(), domainA, events.MemberUpserted{MemberId: "mem_bea"})
	if !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %#v", err)
	}
}

type receivedEnvelope struct {
	EventID    string `json:"eventId"`
	Type       string `json:"type"`
	DomainId   string `json:"domainId"`
	Producer   string `json:"producer"`
	OccurredAt string `json:"occurredAt"`
	Payload    any    `json:"payload"`
}

func TestReceiveIntoWritesTheReceipt(t *testing.T) {
	f := newFixture(t)
	envelope := receivedEnvelope{EventID: "evt_into", Type: "message.created", DomainId: domainA, Producer: "messaging", OccurredAt: "2026-11-10T12:00:00Z", Payload: message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z")}
	receipt := map[string]any{}
	err := f.service.ReceiveInto(serviceCaller(), envelope, &receipt)
	if err != nil || receipt["eventId"] != "evt_into" || receipt["status"] != "applied" {
		t.Fatalf("got %v %v", receipt, err)
	}
	sameIds(t, f.searchIds(tokenA, "q", "alpha"), "msg_1")
}

func TestReceiveIntoRefusesAnEnvelopeThatIsNotJson(t *testing.T) {
	f := newFixture(t)
	envelope := receivedEnvelope{EventID: "evt_into", Type: "message.created", DomainId: domainA, Producer: "messaging", OccurredAt: "2026-11-10T12:00:00Z", Payload: make(chan int)}
	receipt := events.Receipt{}
	err := f.service.ReceiveInto(serviceCaller(), envelope, &receipt)
	if !errors.As(err, &InvalidRequest{}) || receipt != (events.Receipt{}) {
		t.Fatalf("got %v %#v", receipt, err)
	}
}

func TestReceiveIntoPassesARefusalOn(t *testing.T) {
	f := newFixture(t)
	envelope := receivedEnvelope{EventID: "evt_into", Type: "message.created", DomainId: domainA, Producer: "messaging", OccurredAt: "2026-11-10T12:00:00Z", Payload: map[string]any{"bogus": 1}}
	receipt := events.Receipt{}
	err := f.service.ReceiveInto(serviceCaller(), envelope, &receipt)
	refusal := InvalidRequest{}
	if !errors.As(err, &refusal) || refusal.Field != "payload" || receipt != (events.Receipt{}) {
		t.Fatalf("got %v %#v", receipt, err)
	}
}

func indexerOver(documents *fakeDocuments) Indexer {
	return Indexer{Storage: IndexStorage{Store: DocumentStore{Documents: documents}}, Builder: DocumentBuilder{}}
}

func TestIndexerIsUnavailableWhenStorageFails(t *testing.T) {
	types := events.Types{}
	created := envelopeWithId("evt_1", types.MessageCreated(), domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	fresh := envelopeWithId("evt_2", types.MessageCreated(), domainA, message("msg_2", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	edited := envelopeWithId("evt_3", types.MessageEdited(), domainA, events.MessageEdited{MessageId: "msg_1", Body: "bravo", EditedAt: "2026-11-06T10:00:00Z"})
	deleted := envelopeWithId("evt_4", types.MessageDeleted(), domainA, events.MessageDeleted{MessageId: "msg_1"})
	cases := []struct {
		name     string
		envelope events.Envelope
		method   string
		skip     int
	}{
		{"create location lookup", fresh, "CollectionExists", 0},
		{"create month lookup", fresh, "Command", 1},
		{"create document insert", fresh, "Insert", 0},
		{"edit location lookup", edited, "Command", 0},
		{"edit find", edited, "FindById", 0},
		{"edit replace", edited, "Patch", 0},
		{"delete location lookup", deleted, "Command", 0},
		{"delete find", deleted, "FindById", 0},
		{"delete replace", deleted, "Patch", 0},
		{"delete bury", deleted, "Insert", 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			documents := newFakeDocuments()
			indexer := indexerOver(documents)
			if err := indexer.Handle(created); err != nil {
				t.Fatal(err)
			}
			documents.failAt(test.method, test.skip)
			err := indexer.Handle(test.envelope)
			if !errors.As(err, &Unavailable{}) || !errors.Is(err, errFakeStore) {
				t.Fatalf("got %#v", err)
			}
		})
	}
}

func TestIndexerIgnoresTypesItDoesNotIndex(t *testing.T) {
	documents := newFakeDocuments()
	err := indexerOver(documents).Handle(envelopeWithId("evt_1", "channel.created", domainA, json.RawMessage(`not json`)))
	if err != nil || len(documents.commands) != 0 || len(documents.exists) != 0 {
		t.Fatalf("got %v %v", err, documents.commands)
	}
}
