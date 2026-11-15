package syncing_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/syncing"
)

func TestAnEmptyCursorAsksForAResetWithTheNewestChange(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	w.record(w.created(w.post(conversation, "hello"), ""))
	answer := w.syncOne(conversation, "")
	if answer.State != "reset" || answer.Next != w.head(conversation) || len(answer.Messages) != 0 || answer.ConversationId != cnvId(conversation) {
		t.Fatalf("got %+v", answer)
	}
}

func TestAConversationWithoutRowsResetsToAnEmptyCursor(t *testing.T) {
	w := newWorld(t)
	answer := w.syncOne(w.conversation(), "")
	if answer.State != "reset" || answer.Next != "" {
		t.Fatalf("got %+v", answer)
	}
}

func TestACursorTheServerDidNotMakeAsksForAReset(t *testing.T) {
	w := newWorld(t)
	if answer := w.syncOne(w.conversation(), "not-a-cursor"); answer.State != "reset" {
		t.Fatalf("got %+v", answer)
	}
}

func TestACursorOlderThanTheLogAsksForAReset(t *testing.T) {
	w := newWorld(t)
	old := core.ChangeCursor{At: time.Date(2026, time.August, 31, 23, 0, 0, 0, time.UTC), RowId: "1"}
	if answer := w.syncOne(w.conversation(), old.Text()); answer.State != "reset" {
		t.Fatalf("got %+v", answer)
	}
}

func TestNoRowAfterTheNewestCursorIsUnchanged(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	w.record(w.created(w.post(conversation, "hello"), ""))
	answer := w.syncOne(conversation, w.head(conversation))
	if answer.State != "unchanged" || answer.Next != w.head(conversation) || len(answer.Messages) != 0 {
		t.Fatalf("got %+v", answer)
	}
}

func TestSyncAnswersEachChangedMessageOnceInItsCurrentStateWithTheCursorRowInTheOverlap(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	first := w.post(conversation, "first")
	w.record(w.created(first, ""))
	cursor := w.head(conversation)
	w.fakes.Clock.Advance(time.Minute)
	second := w.post(conversation, "second")
	w.record(w.created(second, ""))
	second.Body = "second, edited"
	w.fakes.Stores.Messages.Save(second)
	w.record(w.edited(second))
	answer := w.syncOne(conversation, cursor)
	if answer.State != "changed" || fmt.Sprint(bodies(answer)) != "[first second, edited]" || answer.Next != w.head(conversation) || answer.More {
		t.Fatalf("got %+v", answer)
	}
}

func TestTheOverlapResendsRowsFromJustBeforeTheCursor(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	w.record(w.created(w.post(conversation, "first"), ""))
	cursor := w.head(conversation)
	w.record(w.created(w.post(conversation, "second"), ""))
	answer := w.syncOne(conversation, cursor)
	if fmt.Sprint(bodies(answer)) != "[first second]" {
		t.Fatalf("got %v", bodies(answer))
	}
}

func TestMessagesComeInTheOrderOfTheirLastChange(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	first := w.post(conversation, "first")
	second := w.post(conversation, "second")
	w.record(w.created(first, ""))
	w.record(w.created(second, ""))
	w.record(w.edited(first))
	answer := w.syncOne(conversation, (core.ChangeCursor{At: w.fakes.Clock.Now().Add(-time.Hour), RowId: "0"}).Text())
	if fmt.Sprint(bodies(answer)) != "[second first]" {
		t.Fatalf("got %v", bodies(answer))
	}
}

func TestAPageHoldsAtMostTwoHundredMessages(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	start := (core.ChangeCursor{At: w.fakes.Clock.Now().Add(-time.Hour), RowId: "0"}).Text()
	for index := 0; index < 201; index++ {
		w.record(w.created(w.post(conversation, fmt.Sprint(index)), ""))
	}
	first := w.syncOne(conversation, start)
	second := w.syncOne(conversation, first.Next)
	if len(first.Messages) != 200 || !first.More || second.More || !slices.Contains(bodies(second), "200") || second.Next != w.head(conversation) {
		t.Fatalf("got %d messages (more %v), then %v", len(first.Messages), first.More, bodies(second))
	}
}

func TestADeletedMessageComesWithoutItsBody(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	message := w.post(conversation, "")
	message.DeletedAt = "2026-11-10T12:00:00Z"
	w.fakes.Stores.Messages.Save(message)
	w.record(w.envelope(events.Types{}.MessageDeleted(), events.MessageDeleted{MessageId: msgId(message), ConversationId: cnvId(conversation)}))
	answer := w.syncOne(conversation, (core.ChangeCursor{At: w.fakes.Clock.Now().Add(-time.Hour), RowId: "0"}).Text())
	if len(answer.Messages) != 1 || answer.Messages[0].Message.DeletedAt == "" || answer.Messages[0].Message.Body != "" {
		t.Fatalf("got %+v", answer.Messages)
	}
}

func TestRowsOfMissingOrForeignMessagesAreSkipped(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	other := w.conversation()
	foreign := w.post(other, "elsewhere")
	w.record(w.envelope(events.Types{}.MessageEdited(), events.MessageEdited{MessageId: msgId(foreign), ConversationId: cnvId(conversation)}))
	w.record(w.envelope(events.Types{}.MessageEdited(), events.MessageEdited{MessageId: "msg_202611999", ConversationId: cnvId(conversation)}))
	w.record(w.envelope(events.Types{}.MessageEdited(), events.MessageEdited{MessageId: "not-a-message", ConversationId: cnvId(conversation)}))
	answer := w.syncOne(conversation, (core.ChangeCursor{At: w.fakes.Clock.Now().Add(-time.Hour), RowId: "0"}).Text())
	if answer.State != "changed" || len(answer.Messages) != 0 {
		t.Fatalf("got %+v", answer)
	}
}

func TestUnknownAndForeignConversationsAreGoneWithoutFailingTheBatch(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	answers := w.sync("bo", core.SyncCursor{ConversationId: "cnv_unknown"}, core.SyncCursor{ConversationId: "nonsense"}, core.SyncCursor{ConversationId: cnvId(conversation)})
	if answers[0].State != "gone" || answers[1].State != "gone" || answers[2].State != "reset" {
		t.Fatalf("got %+v", answers)
	}
	stranger, err := syncing.NewSync(w.fakes.Stores, w.fakes.Ports).Run(w.caller("eve"), []core.SyncCursor{{ConversationId: cnvId(conversation)}})
	if err != nil || stranger[0].State != "gone" {
		t.Fatalf("got %+v %v", stranger, err)
	}
}

func TestASyncTakesOneToFiftyCursors(t *testing.T) {
	w := newWorld(t)
	sync := syncing.NewSync(w.fakes.Stores, w.fakes.Ports)
	many := make([]core.SyncCursor, 51)
	for _, cursors := range [][]core.SyncCursor{nil, many} {
		var invalid *core.InvalidRequest
		if _, err := sync.Run(w.caller("bo"), cursors); !errors.As(err, &invalid) || invalid.Field != "cursors" {
			t.Fatalf("want an invalid request for %d cursors, got %v", len(cursors), err)
		}
	}
}

func TestStoreFailuresMakeTheSyncUnavailable(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	w.record(w.created(w.post(conversation, "hello"), ""))
	cursor := (core.ChangeCursor{At: w.fakes.Clock.Now().Add(-time.Hour), RowId: "0"}).Text()
	rows := w.fakes.Stores
	rows.Changes = failingChanges{rows.Changes}
	heads := w.fakes.Stores
	heads.ChangeHeads = failingHeads{ChangeHeadStore: heads.ChangeHeads}
	messages := w.fakes.Stores
	messages.Messages = failingMessages{messages.Messages}
	conversations := w.fakes.Stores
	conversations.Participants = failingParticipants{conversations.Participants}
	cases := []struct {
		stores core.Stores
		after  string
	}{{rows, cursor}, {heads, ""}, {messages, cursor}, {conversations, cursor}}
	for index, check := range cases {
		_, err := syncing.NewSync(check.stores, w.fakes.Ports).Run(w.caller("bo"), []core.SyncCursor{{ConversationId: cnvId(conversation), After: check.after}})
		var unavailable *core.Unavailable
		if !errors.As(err, &unavailable) {
			t.Fatalf("case %d: want Unavailable, got %v", index, err)
		}
	}
}

type failingParticipants struct {
	core.ParticipantStore
}

func (failingParticipants) OfMember(domainId string, memberId string) ([]core.Participant, error) {
	return nil, errors.New("participants are down")
}

func TestRowsOlderThanTheOverlapAreNotResent(t *testing.T) {
	w := newWorld(t)
	conversation := w.conversation()
	w.record(w.created(w.post(conversation, "old"), ""))
	w.fakes.Clock.Advance(time.Minute)
	w.record(w.created(w.post(conversation, "recent"), ""))
	cursor := w.head(conversation)
	w.record(w.created(w.post(conversation, "new"), ""))
	if got := fmt.Sprint(bodies(w.syncOne(conversation, cursor))); got != "[recent new]" {
		t.Fatalf("got %s", got)
	}
}

func TestAConversationOverAMonthBoundaryReadsBothMonths(t *testing.T) {
	w := newWorld(t)
	w.fakes.Clock.At = time.Date(2026, time.November, 30, 23, 59, 58, 0, time.UTC)
	conversation := w.conversation()
	w.record(w.created(w.post(conversation, "november"), ""))
	cursor := w.head(conversation)
	w.fakes.Clock.Advance(time.Hour)
	w.record(w.created(w.post(conversation, "december"), ""))
	if got := fmt.Sprint(bodies(w.syncOne(conversation, cursor))); got != "[november december]" {
		t.Fatalf("got %s", got)
	}
}
