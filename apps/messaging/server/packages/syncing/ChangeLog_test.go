package syncing_test

import (
	"errors"
	"testing"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/syncing"
)

func TestACreatedMessageGetsOneRowAndMovesTheHead(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	message := w.post(conversation, "hello")
	w.record(w.created(message, ""))
	rows := w.rows()
	if len(rows) != 1 || rows[0].MessageId != msgId(message) || rows[0].ConversationId != cnvId(conversation) || rows[0].Month != "202611" || rows[0].EventId != "event-1" {
		t.Fatalf("got %+v", rows)
	}
	if w.head(conversation) != (core.ChangeCursors{}).Of(rows[0]).Text() {
		t.Fatalf("the head is %q", w.head(conversation))
	}
}

func TestAReplyAlsoLogsItsThreadRoot(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	root := w.post(conversation, "root")
	reply := w.post(conversation, "reply")
	w.record(w.created(reply, msgId(root)))
	rows := w.rows()
	if len(rows) != 2 || rows[0].MessageId != msgId(reply) || rows[1].MessageId != msgId(root) {
		t.Fatalf("got %+v", rows)
	}
	if w.head(conversation) != (core.ChangeCursors{}).Of(rows[1]).Text() {
		t.Fatal("the head is not the newest row")
	}
}

func TestARepeatedEventAddsNoRow(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	envelope := w.created(w.post(conversation, "hello"), "")
	w.record(envelope)
	w.record(envelope)
	w.fakes.Clock.Advance(time.Minute)
	again := syncing.NewChangeLog(outbox.PushFunc(func(outbox.Subscriber, events.Envelope) error { return nil }), w.fakes.Stores, w.fakes.Clock.Now)
	if err := again.Record(envelope); err != nil || len(w.rows()) != 1 || len(w.fakes.Stores.ChangeHeads.(*core.MemoryChangeHeads).All()) != 1 {
		t.Fatalf("got %d rows (%v)", len(w.rows()), err)
	}
}

func TestEditsDeletionsAndReactionsAreLogged(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	message := w.post(conversation, "hello")
	payload := events.MessageDeleted{MessageId: msgId(message), ConversationId: cnvId(conversation)}
	w.record(w.edited(message))
	w.record(w.envelope(events.Types{}.MessageDeleted(), payload))
	w.record(w.envelope(events.Types{}.ReactionChanged(), events.ReactionChanged{MessageId: msgId(message), ConversationId: cnvId(conversation)}))
	if len(w.rows()) != 3 {
		t.Fatalf("got %d rows", len(w.rows()))
	}
}

func TestOtherEventsAreOnlyPushed(t *testing.T) {
	w := newWorld(t)
	envelope := w.envelope(events.Types{}.ReadPositionMoved(), events.ReadPositionMoved{ConversationId: "cnv_1"})
	if err := w.log.Push(outbox.Subscriber{Name: "realtime"}, envelope); err != nil {
		t.Fatal(err)
	}
	if len(w.rows()) != 0 || len(w.pushed) != 1 || w.pushed[0] != "realtime/event-1" {
		t.Fatalf("got %v %v", w.rows(), w.pushed)
	}
}

func TestTheRowIsWrittenBeforeEachPush(t *testing.T) {
	w := newWorld(t)
	envelope := w.created(w.post(w.conversation(), "hello"), "")
	w.log.Push(outbox.Subscriber{Name: "realtime"}, envelope)
	w.log.Push(outbox.Subscriber{Name: "notify"}, envelope)
	if len(w.rows()) != 1 || len(w.pushed) != 2 {
		t.Fatalf("got %d rows and %v", len(w.rows()), w.pushed)
	}
}

func TestAPayloadThatIsNotJsonIsRefused(t *testing.T) {
	w := newWorld(t)
	envelope := w.envelope(events.Types{}.MessageEdited(), nil)
	envelope.Payload = []byte("{")
	if err := w.log.Push(outbox.Subscriber{Name: "realtime"}, envelope); err == nil || len(w.pushed) != 0 {
		t.Fatalf("want a refusal before the push, got %v", err)
	}
}

func TestAnUnreadableOccurredAtUsesTheCurrentMonth(t *testing.T) {
	w := newWorld(t)
	envelope := w.created(w.post(w.conversation(), "hello"), "")
	envelope.OccurredAt = "yesterday"
	w.record(envelope)
	if len(w.rows()) != 1 {
		t.Fatalf("got %d rows", len(w.rows()))
	}
}

func TestStoreFailuresStopThePushAsUnavailable(t *testing.T) {
	breaks := map[string]func(stores core.Stores) core.Stores{
		"reads": func(stores core.Stores) core.Stores {
			stores.Changes = failingChanges{stores.Changes}
			return stores
		},
		"appends": func(stores core.Stores) core.Stores {
			stores.Changes = failingAppends{stores.Changes}
			return stores
		},
		"head finds": func(stores core.Stores) core.Stores {
			stores.ChangeHeads = failingHeads{ChangeHeadStore: stores.ChangeHeads}
			return stores
		},
		"head inserts": func(stores core.Stores) core.Stores {
			stores.ChangeHeads = failingHeads{ChangeHeadStore: stores.ChangeHeads, saves: true}
			return stores
		},
	}
	for name, broken := range breaks {
		w := newWorld(t)
		log := syncing.NewChangeLog(outbox.PushFunc(func(outbox.Subscriber, events.Envelope) error { return nil }), broken(w.fakes.Stores), w.fakes.Clock.Now)
		envelope := w.created(w.post(w.conversation(), "hello"), "")
		w.fakes.Clock.Advance(time.Minute)
		var unavailable *core.Unavailable
		if err := log.Record(envelope); !errors.As(err, &unavailable) {
			t.Fatalf("%s: want Unavailable, got %v", name, err)
		}
	}
}

func TestTheHeadNeverMovesBack(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	first := w.created(w.post(conversation, "one"), "")
	w.record(w.created(w.post(conversation, "two"), ""))
	newest := w.head(conversation)
	w.fakes.Clock.Advance(-10 * 1e9)
	if err := w.log.Record(first); err != nil {
		t.Fatal(err)
	}
	if w.head(conversation) != newest {
		t.Fatalf("the head moved back to %q", w.head(conversation))
	}
}

func TestParallelPushesOfOneEventLogItOnce(t *testing.T) {
	w := newWorld(t)
	envelope := w.created(w.post(w.conversation(), "hello"), "")
	done := make(chan error, 4)
	for _, name := range []string{"realtime", "notify", "search", "mail-out"} {
		go func(name string) {
			done <- w.log.Record(envelope)
		}(name)
	}
	for range 4 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if len(w.rows()) != 1 {
		t.Fatalf("got %d rows", len(w.rows()))
	}
}

func TestAFreshEventIsWrittenWithoutLookingForItsRows(t *testing.T) {
	w := newWorld(t)
	stores := w.fakes.Stores
	stores.Changes = failingReads{stores.Changes}
	log := syncing.NewChangeLog(outbox.PushFunc(func(outbox.Subscriber, events.Envelope) error { return nil }), stores, w.fakes.Clock.Now)
	if err := log.Record(w.created(w.post(w.conversation(), "hello"), "")); err != nil || len(w.rows()) != 1 {
		t.Fatalf("got %d rows (%v)", len(w.rows()), err)
	}
}

type failingReads struct {
	core.ChangeStore
}

func (failingReads) OfEvent(domainId string, month string, eventId string) ([]core.Change, error) {
	return nil, errors.New("a fresh event must not be looked up")
}
