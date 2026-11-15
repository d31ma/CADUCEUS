package reading_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/reading"
)

func (s *scene) channelChat() (core.Conversation, core.Message) {
	conversation := s.conversation(core.Conversation{Kind: "channel", Key: "chan1", ChannelId: "chan1", Visibility: "public", MessageCount: 1, CreatedMonth: "202611"})
	s.join(conversation, core.Participant{MemberId: "ana", DisplayName: "Ana"})
	s.join(conversation, core.Participant{MemberId: "bo", DisplayName: "Bo"})
	message := s.post(conversation, core.Message{Sequence: 1, AuthorMemberId: "ana", Body: "hello", BlocksJson: `[{"kind":"paragraph","spans":[{"text":"hello"}]}]`})
	return conversation, message
}

func readersOf(t *testing.T, fields core.OutboxFields, eventType string) []string {
	t.Helper()
	envelopes := staged(t, fields, eventType)
	payload := struct {
		MemberIds []string `json:"memberIds"`
	}{}
	decode(t, envelopes[len(envelopes)-1], &payload)
	return payload.MemberIds
}

func TestAnEditInAChannelListsNoReaders(t *testing.T) {
	s := newScene(t)
	_, message := s.channelChat()
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text", 0)
	wantNoError(t, err)
	readers := readersOf(t, s.stored(message).OutboxFields, events.Types{}.MessageEdited())
	if readers == nil || len(readers) != 0 {
		t.Fatalf("got %v", readers)
	}
}

func TestADeleteInAChannelListsNoReaders(t *testing.T) {
	s := newScene(t)
	_, message := s.channelChat()
	wantNoError(t, reading.NewChanges(s.stores, s.world.Ports).Delete(s.caller("ana"), msgId(message)))
	readers := readersOf(t, s.stored(message).OutboxFields, events.Types{}.MessageDeleted())
	if readers == nil || len(readers) != 0 {
		t.Fatalf("got %v", readers)
	}
}

func TestAReactionInAChannelListsNoReaders(t *testing.T) {
	s := newScene(t)
	_, message := s.channelChat()
	wantNoError(t, reading.NewReactions(s.stores, s.world.Ports).Add(s.caller("bo"), msgId(message), "thumbs_up"))
	readers := readersOf(t, s.reactions()[0].OutboxFields, events.Types{}.ReactionChanged())
	if readers == nil || len(readers) != 0 {
		t.Fatalf("got %v", readers)
	}
}

func TestAnEditInADirectConversationListsItsMembers(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text", 0)
	wantNoError(t, err)
	readers := readersOf(t, s.stored(message).OutboxFields, events.Types{}.MessageEdited())
	if len(readers) != 2 {
		t.Fatalf("got %v", readers)
	}
}

func TestChangesInAChannelOfTenThousandMembersKeepEveryDocumentSmall(t *testing.T) {
	s := newScene(t)
	conversation, message := s.channelChat()
	for index := range 10000 {
		s.join(conversation, core.Participant{MemberId: fmt.Sprintf("%08d-0000-4000-8000-000000000000", index)})
	}
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text", 0)
	wantNoError(t, err)
	wantNoError(t, reading.NewReactions(s.stores, s.world.Ports).Add(s.caller("bo"), msgId(message), "thumbs_up"))
	edited, _ := json.Marshal(s.stored(message))
	reacted, _ := json.Marshal(s.reactions()[0])
	editReaders := readersOf(t, s.stored(message).OutboxFields, events.Types{}.MessageEdited())
	reactionReaders := readersOf(t, s.reactions()[0].OutboxFields, events.Types{}.ReactionChanged())
	if len(edited) >= 16*1024 || len(reacted) >= 16*1024 || len(editReaders) != 0 || len(reactionReaders) != 0 {
		t.Fatalf("the message has %d bytes, the reaction %d, readers %d and %d", len(edited), len(reacted), len(editReaders), len(reactionReaders))
	}
}
