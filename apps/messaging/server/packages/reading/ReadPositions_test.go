package reading_test

import (
	"fmt"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/reading"
)

func (s *scene) positions() (core.Conversation, core.Participant, core.Message, core.Message) {
	conversation := s.conversation(core.Conversation{MessageCount: 2})
	ana := s.join(conversation, core.Participant{MemberId: "ana"})
	first := s.post(conversation, core.Message{Sequence: 1})
	second := s.post(conversation, core.Message{Sequence: 2})
	return conversation, ana, first, second
}

func (s *scene) participant(id string) core.Participant {
	for _, row := range s.participants() {
		if row.Id == id {
			return row
		}
	}
	return core.Participant{}
}

func TestTheReadPositionMovesForward(t *testing.T) {
	s := newScene(t)
	conversation, ana, _, second := s.positions()
	wantNoError(t, reading.NewReadPositions(s.stores, s.world.Ports).Move(s.caller("ana"), cnvId(conversation), msgId(second)))
	moved := s.participant(ana.Id)
	envelopes := staged(t, moved.OutboxFields, events.Types{}.ReadPositionMoved())
	payload := events.ReadPositionMoved{}
	decode(t, envelopes[0], &payload)
	want := events.ReadPositionMoved{ConversationId: cnvId(conversation), MemberId: "ana", MessageId: msgId(second), ReadAt: "2026-11-10T12:00:00Z"}
	got := fmt.Sprint(moved.ReadSequence, moved.ReadMessageId, moved.ReadAt, payload)
	if got != fmt.Sprint(2, msgId(second), "2026-11-10T12:00:00Z", want) || len(envelopes) != 1 {
		t.Fatalf("got %s", got)
	}
}

func TestTheReadPositionNeverMovesBack(t *testing.T) {
	s := newScene(t)
	conversation, ana, first, second := s.positions()
	positions := reading.NewReadPositions(s.stores, s.world.Ports)
	wantNoError(t, positions.Move(s.caller("ana"), cnvId(conversation), msgId(second)))
	wantNoError(t, positions.Move(s.caller("ana"), cnvId(conversation), msgId(first)))
	wantNoError(t, positions.Move(s.caller("ana"), cnvId(conversation), msgId(second)))
	moved := s.participant(ana.Id)
	if moved.ReadSequence != 2 || len(staged(t, moved.OutboxFields, events.Types{}.ReadPositionMoved())) != 1 {
		t.Fatalf("got %+v", moved)
	}
}

func TestAReaderWhoIsNotAParticipantMovesNothing(t *testing.T) {
	s := newScene(t)
	channel := s.conversation(core.Conversation{Kind: "channel", Key: "chan1", Visibility: "public", MessageCount: 1})
	message := s.post(channel, core.Message{Sequence: 1})
	wantNoError(t, reading.NewReadPositions(s.stores, s.world.Ports).Move(s.caller("bo"), cnvId(channel), msgId(message)))
	if len(s.participants()) != 0 {
		t.Fatalf("got %+v", s.participants())
	}
}

func TestTheReadMessageMustBelongToTheConversation(t *testing.T) {
	s := newScene(t)
	conversation, _, _, _ := s.positions()
	other := s.conversation(core.Conversation{})
	s.join(other, core.Participant{MemberId: "ana"})
	elsewhere := s.post(other, core.Message{Sequence: 9})
	wantError[*core.NotFound](t, reading.NewReadPositions(s.stores, s.world.Ports).Move(s.caller("ana"), cnvId(conversation), msgId(elsewhere)))
}

func TestTheReadPositionInAnotherDomainIsNotFound(t *testing.T) {
	s := newScene(t)
	conversation, _, _, second := s.positions()
	caller := s.caller("ana")
	caller.DomainId = "example.org"
	wantError[*core.NotFound](t, reading.NewReadPositions(s.stores, s.world.Ports).Move(caller, cnvId(conversation), msgId(second)))
}

func TestTheReadPositionReportsABrokenSave(t *testing.T) {
	s := newScene(t)
	conversation, _, _, second := s.positions()
	s.breaker.broken["participants.save"] = true
	wantError[*core.Unavailable](t, reading.NewReadPositions(s.stores, s.world.Ports).Move(s.caller("ana"), cnvId(conversation), msgId(second)))
}

func TestTheReadPositionReportsABrokenOutbox(t *testing.T) {
	s := newScene(t)
	conversation, ana, _, second := s.positions()
	ana.OutboxFields = corrupt()
	s.world.Stores.Participants.Save(ana)
	err := reading.NewReadPositions(s.stores, s.world.Ports).Move(s.caller("ana"), cnvId(conversation), msgId(second))
	if err == nil || s.participant(ana.Id).ReadSequence != 0 {
		t.Fatalf("got %v", err)
	}
}

func TestTheReadPositionReportsABrokenParticipantStore(t *testing.T) {
	s := newScene(t)
	conversation, _, _, second := s.positions()
	s.breaker.broken["participants.ofMember"] = true
	wantError[*core.Unavailable](t, reading.NewReadPositions(s.stores, s.world.Ports).Move(s.caller("ana"), cnvId(conversation), msgId(second)))
}
