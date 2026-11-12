package outbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"mesail.dev/packages/events"
)

func messageSettings() Settings {
	return Settings{
		Producer: "messaging",
		Subscribers: []Subscriber{
			{Name: "realtime", URL: "http://realtime.test/events"},
			{Name: "notify", URL: "http://notify.test/events"},
			{Name: "search", URL: "http://search.test/events"},
		},
	}
}

func messageEvent(eventId string, eventType string, payload string) events.Envelope {
	return events.Envelope{
		EventID:    eventId,
		Type:       eventType,
		DomainId:   "example.com",
		Producer:   "messaging",
		OccurredAt: "2026-11-12T12:00:00Z",
		Payload:    json.RawMessage(payload),
	}
}

func edited(eventId string, messageId string, body string) events.Envelope {
	return messageEvent(eventId, events.Types{}.MessageEdited(), `{"messageId":"`+messageId+`","body":"`+body+`"}`)
}

func pendingIds(pending []PendingEvent) []string {
	ids := []string{}
	for _, event := range pending {
		ids = append(ids, event.Envelope.EventID)
	}
	return ids
}

func fieldsWith(t *testing.T, state OutboxState) Fields {
	t.Helper()
	fields, err := Codec{}.Write(state)
	if err != nil {
		t.Fatal(err)
	}
	return fields
}

func TestANewerEditReplacesTheWaitingEditOfTheSameMessage(t *testing.T) {
	fields := stageAll(t, messageSettings(), edited("evt_1", "msg_1", "one"), edited("evt_2", "msg_1", "two"))

	pending := pendingOf(t, fields)
	if !reflect.DeepEqual(pendingIds(pending), []string{"evt_2"}) || fields.OutboxPending != 1 {
		t.Fatalf("pending %v, fields %+v", pendingIds(pending), fields)
	}
}

func TestEditsOfDifferentMessagesBothWait(t *testing.T) {
	fields := stageAll(t, messageSettings(), edited("evt_1", "msg_1", "one"), edited("evt_2", "msg_2", "two"))

	if got := pendingIds(pendingOf(t, fields)); !reflect.DeepEqual(got, []string{"evt_1", "evt_2"}) {
		t.Fatalf("pending %v", got)
	}
}

func TestEventsWithoutAReplaceKeyAreAddedEachTime(t *testing.T) {
	created := events.Types{}.MessageCreated()
	fields := stageAll(t, messageSettings(), messageEvent("evt_1", created, `{"messageId":"msg_1"}`), messageEvent("evt_2", created, `{"messageId":"msg_1"}`))

	if got := pendingIds(pendingOf(t, fields)); !reflect.DeepEqual(got, []string{"evt_1", "evt_2"}) {
		t.Fatalf("pending %v", got)
	}
}

func TestTheReplacingEventWaitsForTheUnionOfSubscribers(t *testing.T) {
	waiting := PendingEvent{Envelope: edited("evt_1", "msg_1", "one"), Waiting: []string{"search", "archive"}, Attempts: 4}
	before := fieldsWith(t, OutboxState{Pending: []PendingEvent{waiting}, Dead: []DeadLetter{}})

	fields, err := NewOutbox(messageSettings()).Stage(before, edited("evt_2", "msg_1", "two"))

	pending := pendingOf(t, fields)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending %+v, %v", pending, err)
	}
	if want := []string{"search", "archive", "realtime", "notify"}; !reflect.DeepEqual(pending[0].Waiting, want) {
		t.Fatalf("waiting %v", pending[0].Waiting)
	}
	if pending[0].Envelope.EventID != "evt_2" || pending[0].Attempts != 0 {
		t.Fatalf("event %+v", pending[0])
	}
}

func TestANewerReactionReplacesTheWaitingOne(t *testing.T) {
	reaction := events.Types{}.ReactionChanged()
	fields := stageAll(t, messageSettings(),
		messageEvent("evt_1", reaction, `{"messageId":"msg_1","added":true}`),
		messageEvent("evt_2", reaction, `{"messageId":"msg_1","added":false}`))

	if got := pendingIds(pendingOf(t, fields)); !reflect.DeepEqual(got, []string{"evt_2"}) {
		t.Fatalf("pending %v", got)
	}
}

func TestADeleteDropsTheWaitingEditsOfItsMessageOnly(t *testing.T) {
	types := events.Types{}
	fields := stageAll(t, messageSettings(),
		messageEvent("evt_1", types.MessageCreated(), `{"messageId":"msg_1"}`),
		edited("evt_2", "msg_1", "one"),
		edited("evt_3", "msg_2", "other"),
		messageEvent("evt_4", types.MessageDeleted(), `{"messageId":"msg_1"}`))

	if got := pendingIds(pendingOf(t, fields)); !reflect.DeepEqual(got, []string{"evt_1", "evt_3", "evt_4"}) {
		t.Fatalf("pending %v", got)
	}
}

func TestADeleteDropsWaitingEditsEvenWithNoSubscriberForItself(t *testing.T) {
	waiting := PendingEvent{Envelope: edited("evt_1", "msg_1", "one"), Waiting: []string{"search"}}
	before := fieldsWith(t, OutboxState{Pending: []PendingEvent{waiting}, Dead: []DeadLetter{}})
	deleted := messageEvent("evt_2", events.Types{}.MessageDeleted(), `{"messageId":"msg_1"}`)

	fields, err := NewOutbox(Settings{Producer: "messaging"}).Stage(before, deleted)

	if err != nil || fields.OutboxPending != 0 || fields.OutboxWaiting {
		t.Fatalf("fields %+v, %v", fields, err)
	}
}

func TestStageRefusesAWaitingKeyedEventItCannotRead(t *testing.T) {
	broken := PendingEvent{Envelope: messageEvent("evt_1", events.Types{}.MessageEdited(), `[1]`), Waiting: []string{"search"}}
	before := fieldsWith(t, OutboxState{Pending: []PendingEvent{broken}, Dead: []DeadLetter{}})

	fields, err := NewOutbox(messageSettings()).Stage(before, edited("evt_2", "msg_1", "two"))

	if err == nil || !reflect.DeepEqual(fields, before) {
		t.Fatalf("fields %+v, %v", fields, err)
	}
}

func TestStageRefusesANewKeyedEventItCannotRead(t *testing.T) {
	_, replaceErr := NewOutbox(messageSettings()).Stage(Fields{}, messageEvent("evt_1", events.Types{}.MessageEdited(), `[1]`))
	_, removeErr := NewOutbox(messageSettings()).Stage(Fields{}, messageEvent("evt_2", events.Types{}.MessageDeleted(), `[1]`))

	if replaceErr == nil || removeErr == nil {
		t.Fatalf("errors %v, %v", replaceErr, removeErr)
	}
}

func TestStageRefusesAWaitingDeleteTargetItCannotRead(t *testing.T) {
	broken := PendingEvent{Envelope: messageEvent("evt_1", events.Types{}.MessageEdited(), `[1]`), Waiting: []string{"search"}}
	before := fieldsWith(t, OutboxState{Pending: []PendingEvent{broken}, Dead: []DeadLetter{}})

	_, err := NewOutbox(messageSettings()).Stage(before, messageEvent("evt_2", events.Types{}.MessageDeleted(), `{"messageId":"msg_1"}`))

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestManyEditsKeepTheOutboxSmall(t *testing.T) {
	body := strings.Repeat("a", 10*1024)
	fields := Fields{}
	for index := 0; index < 1000; index++ {
		staged, err := NewOutbox(messageSettings()).Stage(fields, edited(fmt.Sprintf("evt_%d", index), "msg_1", body))
		if err != nil {
			t.Fatal(err)
		}
		fields = staged
	}
	encoded, _ := json.Marshal(fields)
	if fields.OutboxPending != 1 || len(encoded) > 16*1024 {
		t.Fatalf("pending %d, %d bytes", fields.OutboxPending, len(encoded))
	}
}

func TestStageRefusesAnEventThatWouldPassTheDocumentCap(t *testing.T) {
	before := stageAll(t, messageSettings(), edited("evt_1", "msg_1", "small"))
	huge := strings.Repeat("a", 512*1024)

	fields, err := NewOutbox(messageSettings()).Stage(before, edited("evt_2", "msg_2", huge))

	var full OutboxFull
	if !errors.As(err, &full) || full.Limit != 512*1024 || full.Bytes <= full.Limit {
		t.Fatalf("got %v", err)
	}
	if !reflect.DeepEqual(fields, before) {
		t.Fatalf("fields changed: %d pending", fields.OutboxPending)
	}
}

func TestOutboxFullExplainsItself(t *testing.T) {
	want := "the document's waiting events would take 600000 bytes, over the 524288 byte document cap; try again later"
	if got := (OutboxFull{Bytes: 600000, Limit: 524288}).Error(); got != want {
		t.Fatalf("got %q", got)
	}
}

func deadLetter(envelope events.Envelope, subscriber string) DeadLetter {
	return DeadLetter{Envelope: envelope, Subscriber: subscriber, Attempts: 8, LastError: "down", DeadAt: "2026-11-12T12:00:00Z"}
}

func deadIds(dead []DeadLetter) []string {
	ids := []string{}
	for _, letter := range dead {
		ids = append(ids, letter.Envelope.EventID+"/"+letter.Subscriber)
	}
	return ids
}

func TestANewerDeadLetterReplacesTheOlderOneForThatSubscriber(t *testing.T) {
	created := messageEvent("evt_0", events.Types{}.MessageCreated(), `{"messageId":"msg_1"}`)
	old := []DeadLetter{deadLetter(created, "search"), deadLetter(edited("evt_1", "msg_1", "one"), "search"), deadLetter(edited("evt_1", "msg_1", "one"), "notify")}
	fresh := fieldsWith(t, OutboxState{Pending: []PendingEvent{}, Dead: old})
	outcome := runOutcome{updated: map[string]PendingEvent{}, deadLetters: []DeadLetter{deadLetter(edited("evt_2", "msg_1", "two"), "search"), deadLetter(created, "realtime")}}

	merged, err := merge{outcome: outcome}.apply(fresh)

	state, readErr := Codec{}.Read(merged)
	want := []string{"evt_0/search", "evt_1/notify", "evt_2/search", "evt_0/realtime"}
	if err != nil || readErr != nil || !reflect.DeepEqual(deadIds(state.Dead), want) {
		t.Fatalf("dead %v, %v, %v", deadIds(state.Dead), err, readErr)
	}
}

func TestMergeRefusesADeadLetterItCannotRead(t *testing.T) {
	broken := deadLetter(messageEvent("evt_1", events.Types{}.MessageEdited(), `[1]`), "search")
	fresh := fieldsWith(t, OutboxState{Pending: []PendingEvent{}, Dead: []DeadLetter{broken}})
	newLetter := runOutcome{updated: map[string]PendingEvent{}, deadLetters: []DeadLetter{deadLetter(edited("evt_2", "msg_1", "two"), "search")}}
	brokenLetter := runOutcome{updated: map[string]PendingEvent{}, deadLetters: []DeadLetter{broken}}

	_, oldErr := merge{outcome: newLetter}.apply(fieldsWith(t, OutboxState{Pending: []PendingEvent{}, Dead: []DeadLetter{broken}}))
	_, newErr := merge{outcome: brokenLetter}.apply(fieldsWith(t, OutboxState{Pending: []PendingEvent{}, Dead: []DeadLetter{}}))

	if oldErr == nil || newErr == nil || fresh.OutboxDead != 1 {
		t.Fatalf("errors %v, %v", oldErr, newErr)
	}
}

func TestStageRefusesAPayloadItCannotEncode(t *testing.T) {
	broken := messageEvent("evt_1", events.Types{}.MessageCreated(), `{`)

	fields, err := NewOutbox(messageSettings()).Stage(Fields{}, broken)

	if err == nil || fields.OutboxPending != 0 {
		t.Fatalf("fields %+v, %v", fields, err)
	}
}
