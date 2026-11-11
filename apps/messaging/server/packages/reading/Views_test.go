package reading_test

import (
	"fmt"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/reading"
)

func (s *scene) delivered() (core.Conversation, core.Message) {
	conversation := s.conversation(core.Conversation{MessageCount: 2})
	s.join(conversation, core.Participant{MemberId: "ana", ReadSequence: 2})
	s.join(conversation, core.Participant{MemberId: "bo", ReadSequence: 2})
	s.join(conversation, core.Participant{MemberId: "cy", ReadSequence: 1})
	deliveries := core.DeliveryCodec{}.Encode([]core.DeliveryRecord{
		{Recipient: "bo@example.com", MemberId: "bo", Channel: "chat", Role: "to", Status: "delivered"},
		{Recipient: "cy@example.com", MemberId: "cy", Channel: "chat", Role: "to", Status: "delivered"},
		{Recipient: "x@ext.org", Channel: "email", Role: "to", Status: "sent"},
		{Recipient: "hidden@ext.org", Channel: "email", Role: "bcc", Status: "queued"},
	})
	message := s.post(conversation, core.Message{Sequence: 2, AuthorMemberId: "ana", Deliveries: deliveries})
	return conversation, message
}

func statusesOf(view core.MessageView) []string {
	statuses := []string{}
	for _, record := range view.Deliveries {
		statuses = append(statuses, record.Recipient+"="+record.Status)
	}
	return statuses
}

func TestViewsShowBccOnlyToTheAuthor(t *testing.T) {
	s := newScene(t)
	conversation, message := s.delivered()
	views := reading.NewViews(s.stores)
	author, err := views.Of(s.caller("ana"), []core.Message{message}, conversation)
	wantNoError(t, err)
	reader, err := views.Of(s.caller("bo"), []core.Message{message}, conversation)
	wantNoError(t, err)
	if len(author[0].Deliveries) != 4 || len(reader[0].Deliveries) != 3 {
		t.Fatalf("author %v, reader %v", statusesOf(author[0]), statusesOf(reader[0]))
	}
}

func TestViewsMarkChatDeliveriesRead(t *testing.T) {
	s := newScene(t)
	conversation, message := s.delivered()
	views, err := reading.NewViews(s.stores).Of(s.caller("bo"), []core.Message{message}, conversation)
	wantNoError(t, err)
	want := []string{"bo@example.com=read", "cy@example.com=delivered", "x@ext.org=sent"}
	if fmt.Sprint(statusesOf(views[0])) != fmt.Sprint(want) {
		t.Fatalf("got %v", statusesOf(views[0]))
	}
}

func TestViewsCountActiveReactions(t *testing.T) {
	s := newScene(t)
	conversation, message := s.chat()
	other := s.post(conversation, core.Message{Sequence: 2})
	add := func(memberId string, emoji string, target core.Message, active bool) {
		s.world.Stores.Reactions.Insert(core.Reaction{DomainId: "example.com", ConversationId: cnvId(conversation), MessageId: msgId(target), MemberId: memberId, Emoji: emoji, Active: active})
	}
	add("ana", "+1", message, true)
	add("bo", "+1", message, true)
	add("bo", "tada", message, false)
	add("ana", "tada", other, true)
	views, err := reading.NewViews(s.stores).Of(s.caller("ana"), []core.Message{message}, conversation)
	wantNoError(t, err)
	got := fmt.Sprint(views[0].Reactions, views[0].MyReactions)
	if got != "map[+1:2] [+1]" {
		t.Fatalf("got %s", got)
	}
}

func TestViewsDecodeBlocks(t *testing.T) {
	s := newScene(t)
	conversation, message := s.chat()
	views, err := reading.NewViews(s.stores).Of(s.caller("ana"), []core.Message{message}, conversation)
	wantNoError(t, err)
	if len(views[0].Blocks) != 1 || views[0].Blocks[0].Spans[0].Text != "hello" {
		t.Fatalf("got %+v", views[0].Blocks)
	}
}

func TestViewsShowNoBlocksForADeletedMessage(t *testing.T) {
	s := newScene(t)
	conversation, message := s.chat()
	message.DeletedAt = "2026-11-10T12:00:00Z"
	views, err := reading.NewViews(s.stores).Of(s.caller("ana"), []core.Message{message}, conversation)
	wantNoError(t, err)
	if views[0].Blocks == nil || len(views[0].Blocks) != 0 {
		t.Fatalf("got %+v", views[0].Blocks)
	}
}

func TestViewsShowNoBlocksWhenNoneAreStored(t *testing.T) {
	s := newScene(t)
	conversation, message := s.chat()
	message.BlocksJson = ""
	views, err := reading.NewViews(s.stores).Of(s.caller("ana"), []core.Message{message}, conversation)
	wantNoError(t, err)
	if len(views[0].Blocks) != 0 {
		t.Fatalf("got %+v", views[0].Blocks)
	}
}

func TestViewsReportBrokenBlocks(t *testing.T) {
	s := newScene(t)
	conversation, message := s.chat()
	message.BlocksJson = "{"
	_, err := reading.NewViews(s.stores).Of(s.caller("ana"), []core.Message{message}, conversation)
	if err == nil {
		t.Fatal("broken blocks were accepted")
	}
}

func TestViewsReportBrokenDeliveries(t *testing.T) {
	s := newScene(t)
	conversation, message := s.chat()
	message.Deliveries = []string{"{"}
	_, err := reading.NewViews(s.stores).Of(s.caller("ana"), []core.Message{message}, conversation)
	if err == nil {
		t.Fatal("broken deliveries were accepted")
	}
}
