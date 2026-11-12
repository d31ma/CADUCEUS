package reading_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/messaging"
	"mesail.dev/packages/reading"
)

func reactionPayloads(t *testing.T, row core.Reaction) []string {
	t.Helper()
	payloads := []string{}
	for _, envelope := range staged(t, row.OutboxFields, events.Types{}.ReactionChanged()) {
		payload := events.ReactionChanged{}
		decode(t, envelope, &payload)
		payloads = append(payloads, fmt.Sprint(payload))
	}
	return payloads
}

func TestAddingAReactionTwiceKeepsOneRow(t *testing.T) {
	s := newScene(t)
	conversation, message := s.chat()
	reactions := reading.NewReactions(s.stores, s.world.Ports)
	wantNoError(t, reactions.Add(s.caller("bo"), msgId(message), "thumbs_up"))
	wantNoError(t, reactions.Add(s.caller("bo"), msgId(message), "thumbs_up"))
	rows := s.reactions()
	sum := sha256.Sum256([]byte(msgId(message) + "\nbo\nthumbs_up"))
	want := fmt.Sprint(events.ReactionChanged{MessageId: msgId(message), ConversationId: cnvId(conversation), MemberId: "bo", Emoji: "thumbs_up", Added: true, MemberIds: []string{"ana", "bo"}})
	if len(rows) != 1 || !rows[0].Active || rows[0].Key != hex.EncodeToString(sum[:]) || fmt.Sprint(reactionPayloads(t, rows[0])) != fmt.Sprint([]string{want}) {
		t.Fatalf("got %+v", rows)
	}
}

func TestRemovingAReactionStagesARemoval(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	reactions := reading.NewReactions(s.stores, s.world.Ports)
	wantNoError(t, reactions.Add(s.caller("bo"), msgId(message), "thumbs_up"))
	wantNoError(t, reactions.Remove(s.caller("bo"), msgId(message), "thumbs_up"))
	wantNoError(t, reactions.Remove(s.caller("bo"), msgId(message), "thumbs_up"))
	rows := s.reactions()
	payloads := reactionPayloads(t, rows[0])
	if len(rows) != 1 || rows[0].Active || len(payloads) != 1 || !strings.Contains(payloads[0], " false ") {
		t.Fatalf("got %+v %v", rows, payloads)
	}
}

func TestRemovingAMissingReactionWritesNothing(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	wantNoError(t, reading.NewReactions(s.stores, s.world.Ports).Remove(s.caller("bo"), msgId(message), "thumbs_up"))
	if len(s.reactions()) != 0 {
		t.Fatalf("got %+v", s.reactions())
	}
}

func TestAReactionCanBeAddedAgain(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	reactions := reading.NewReactions(s.stores, s.world.Ports)
	wantNoError(t, reactions.Add(s.caller("bo"), msgId(message), "thumbs_up"))
	wantNoError(t, reactions.Remove(s.caller("bo"), msgId(message), "thumbs_up"))
	wantNoError(t, reactions.Add(s.caller("bo"), msgId(message), "thumbs_up"))
	rows := s.reactions()
	if len(rows) != 1 || !rows[0].Active || len(reactionPayloads(t, rows[0])) != 1 {
		t.Fatalf("got %+v", rows)
	}
}

func TestAReactionNameOutsideTheListIsRefused(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	reactions := reading.NewReactions(s.stores, s.world.Ports)
	for _, name := range []string{"", "+1", "\U0001F44D", "Thumbs_Up", "thumbs up", "unicorn", strings.Repeat("a", 33)} {
		wantInvalid(t, reactions.Add(s.caller("bo"), msgId(message), name), "reaction")
		wantInvalid(t, reactions.Remove(s.caller("bo"), msgId(message), name), "reaction")
	}
	if len(s.reactions()) != 0 {
		t.Fatalf("got %+v", s.reactions())
	}
}

func TestEveryListedReactionNameIsAccepted(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	reactions := reading.NewReactions(s.stores, s.world.Ports)
	for _, name := range (messaging.ReactionNames{}).All() {
		wantNoError(t, reactions.Add(s.caller("bo"), msgId(message), name))
	}
	if len(s.reactions()) != len(messaging.ReactionNames{}.All()) {
		t.Fatalf("got %d rows", len(s.reactions()))
	}
}

func TestAReactionInAnotherDomainIsNotFound(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	caller := s.caller("bo")
	caller.DomainId = "example.org"
	wantError[*core.NotFound](t, reading.NewReactions(s.stores, s.world.Ports).Add(caller, msgId(message), "thumbs_up"))
}

func TestAReactionReportsABrokenLookup(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.breaker.broken["reactions.byKey"] = true
	wantError[*core.Unavailable](t, reading.NewReactions(s.stores, s.world.Ports).Add(s.caller("bo"), msgId(message), "thumbs_up"))
}

func TestAReactionReportsABrokenInsert(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.breaker.broken["reactions.insert"] = true
	wantError[*core.Unavailable](t, reading.NewReactions(s.stores, s.world.Ports).Add(s.caller("bo"), msgId(message), "thumbs_up"))
}

func TestAReactionReportsABrokenSave(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	reactions := reading.NewReactions(s.stores, s.world.Ports)
	wantNoError(t, reactions.Add(s.caller("bo"), msgId(message), "thumbs_up"))
	s.breaker.broken["reactions.save"] = true
	wantError[*core.Unavailable](t, reactions.Remove(s.caller("bo"), msgId(message), "thumbs_up"))
}

func TestAReactionReportsABrokenParticipantList(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.breaker.broken["participants.ofConversation"] = true
	wantError[*core.Unavailable](t, reading.NewReactions(s.stores, s.world.Ports).Add(s.caller("bo"), msgId(message), "thumbs_up"))
}

func TestAReactionReportsABrokenOutbox(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	reactions := reading.NewReactions(s.stores, s.world.Ports)
	wantNoError(t, reactions.Add(s.caller("bo"), msgId(message), "thumbs_up"))
	row := s.reactions()[0]
	row.OutboxFields = corrupt()
	s.world.Stores.Reactions.Save(row)
	if err := reactions.Remove(s.caller("bo"), msgId(message), "thumbs_up"); err == nil || !s.reactions()[0].Active {
		t.Fatalf("got %v", err)
	}
}
