package reading_test

import (
	"fmt"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/reading"
)

func TestAnEditWithTheCurrentVersionCountsUp(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	view, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text", 1)
	wantNoError(t, err)
	if view.Message.Version != 2 || s.stored(message).Version != 2 {
		t.Fatalf("got version %d", view.Message.Version)
	}
}

func TestAnEditWithAnOldVersionIsAConflictNamingTheCurrentMessage(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	changes := reading.NewChanges(s.stores, s.world.Ports)
	_, err := changes.Edit(s.caller("ana"), msgId(message), "from the laptop", 1)
	wantNoError(t, err)
	_, err = changes.Edit(s.caller("ana"), msgId(message), "from the phone", 1)
	conflict := wantError[*core.EditConflict](t, err)
	if conflict.Current.Message.Body != "from the laptop" || conflict.Current.Message.Version != 2 || s.stored(message).Body != "from the laptop" {
		t.Fatalf("got %+v", conflict.Current.Message)
	}
}

func TestADeletedMessageIsAConflictBeforeItsVersionIsCompared(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	changes := reading.NewChanges(s.stores, s.world.Ports)
	wantNoError(t, changes.Delete(s.caller("ana"), msgId(message)))
	_, err := changes.Edit(s.caller("ana"), msgId(message), "too late", 7)
	if wantError[*core.Conflict](t, err).Code != "message_deleted" {
		t.Fatal("want message_deleted")
	}
}

func TestADeletionCountsTheVersionUpOnce(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	changes := reading.NewChanges(s.stores, s.world.Ports)
	wantNoError(t, changes.Delete(s.caller("ana"), msgId(message)))
	wantNoError(t, changes.Delete(s.caller("ana"), msgId(message)))
	if s.stored(message).Version != 2 {
		t.Fatalf("got version %d", s.stored(message).Version)
	}
}

func TestAConflictFailsWhenTheCurrentMessageCannotBeShown(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.breaker.broken["reactions.ofConversation"] = true
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "text", 5)
	wantError[*core.Unavailable](t, err)
}

func TestReactionsOnADeletedMessageAreConflicts(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	wantNoError(t, reading.NewChanges(s.stores, s.world.Ports).Delete(s.caller("ana"), msgId(message)))
	reactions := reading.NewReactions(s.stores, s.world.Ports)
	added := reactions.Add(s.caller("bo"), msgId(message), "heart")
	removed := reactions.Remove(s.caller("bo"), msgId(message), "heart")
	if wantError[*core.Conflict](t, added).Code != "message_deleted" || wantError[*core.Conflict](t, removed).Code != "message_deleted" {
		t.Fatal("want message_deleted")
	}
}

func TestTheListNamesTheNewestChangeAndTheReadPosition(t *testing.T) {
	s := newScene(t)
	conversation := s.conversation(core.Conversation{MessageCount: 1})
	s.join(conversation, core.Participant{MemberId: "ana", ReadMessageId: "msg_202611000000000001"})
	s.world.Stores.ChangeHeads.Insert(core.ChangeHead{DomainId: "example.com", ConversationId: cnvId(conversation), LastChangeId: "0001.000000000001"})
	list, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{})
	wantNoError(t, err)
	got := fmt.Sprint(list.Items[0].LastChangeId, " ", list.Items[0].ReadMessageId)
	if got != "0001.000000000001 msg_202611000000000001" {
		t.Fatalf("got %s", got)
	}
}
