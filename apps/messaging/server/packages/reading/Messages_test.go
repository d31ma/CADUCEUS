package reading_test

import (
	"fmt"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/reading"
)

type history struct {
	conversation core.Conversation
	first        core.Message
	second       core.Message
	third        core.Message
	reply        core.Message
	fifth        core.Message
}

func (s *scene) history() history {
	conversation := s.conversation(core.Conversation{MessageCount: 5, CreatedMonth: "202610"})
	s.join(conversation, core.Participant{MemberId: "ana"})
	s.join(conversation, core.Participant{MemberId: "bo"})
	built := history{conversation: conversation}
	s.post(conversation, core.Message{Month: "202609", Sequence: 0, Body: "before the conversation"})
	built.first = s.post(conversation, core.Message{Month: "202610", Sequence: 1})
	built.second = s.post(conversation, core.Message{Month: "202610", Sequence: 2})
	built.third = s.post(conversation, core.Message{Sequence: 3})
	built.reply = s.post(conversation, core.Message{Sequence: 4, ThreadRootId: msgId(built.first)})
	built.fifth = s.post(conversation, core.Message{Sequence: 5})
	return built
}

func idsOf(list core.MessageList) []string {
	ids := []string{}
	for _, item := range list.Items {
		ids = append(ids, msgId(item.Message))
	}
	return ids
}

func TestMessagesListRootsNewestFirst(t *testing.T) {
	s := newScene(t)
	h := s.history()
	list, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(h.conversation), "", core.PageRequest{})
	wantNoError(t, err)
	want := []string{msgId(h.fifth), msgId(h.third), msgId(h.second), msgId(h.first)}
	if fmt.Sprint(idsOf(list)) != fmt.Sprint(want) || list.NextCursor != "" {
		t.Fatalf("got %v next %q, want %v", idsOf(list), list.NextCursor, want)
	}
}

func TestMessagesArePagedBySequence(t *testing.T) {
	s := newScene(t)
	h := s.history()
	reader := reading.NewReader(s.stores, s.world.Ports)
	first, err := reader.Messages(s.caller("ana"), cnvId(h.conversation), "", core.PageRequest{Limit: 2})
	wantNoError(t, err)
	second, err := reader.Messages(s.caller("ana"), cnvId(h.conversation), "", core.PageRequest{Limit: 2, Cursor: first.NextCursor})
	wantNoError(t, err)
	got := fmt.Sprint(idsOf(first), first.NextCursor, idsOf(second), second.NextCursor == "")
	want := fmt.Sprint([]string{msgId(h.fifth), msgId(h.third)}, "3", []string{msgId(h.second), msgId(h.first)}, true)
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMessagesListTheRepliesOfAThread(t *testing.T) {
	s := newScene(t)
	h := s.history()
	list, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(h.conversation), msgId(h.first), core.PageRequest{})
	wantNoError(t, err)
	if fmt.Sprint(idsOf(list)) != fmt.Sprint([]string{msgId(h.reply)}) {
		t.Fatalf("got %v", idsOf(list))
	}
}

func TestMessagesRefuseARootFromAnotherConversation(t *testing.T) {
	s := newScene(t)
	h := s.history()
	_, message := s.chat()
	_, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(h.conversation), msgId(message), core.PageRequest{})
	wantError[*core.NotFound](t, err)
}

func TestMessagesRefuseAMalformedRoot(t *testing.T) {
	s := newScene(t)
	h := s.history()
	_, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(h.conversation), "msg_x", core.PageRequest{})
	wantError[*core.NotFound](t, err)
}

func TestMessagesRefuseAnotherDomain(t *testing.T) {
	s := newScene(t)
	h := s.history()
	caller := s.caller("ana")
	caller.DomainId = "example.org"
	_, err := reading.NewReader(s.stores, s.world.Ports).Messages(caller, cnvId(h.conversation), "", core.PageRequest{})
	wantError[*core.NotFound](t, err)
}

func TestMessagesRefuseANonParticipant(t *testing.T) {
	s := newScene(t)
	h := s.history()
	_, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("cy"), cnvId(h.conversation), "", core.PageRequest{})
	wantError[*core.NotFound](t, err)
}

func TestMessagesReadAtMostTwentyFourMonths(t *testing.T) {
	s := newScene(t)
	conversation := s.conversation(core.Conversation{})
	s.join(conversation, core.Participant{MemberId: "ana"})
	kept := s.post(conversation, core.Message{Month: "202412", Sequence: 2})
	s.post(conversation, core.Message{Month: "202411", Sequence: 1})
	list, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(conversation), "", core.PageRequest{})
	wantNoError(t, err)
	if fmt.Sprint(idsOf(list)) != fmt.Sprint([]string{msgId(kept)}) {
		t.Fatalf("got %v", idsOf(list))
	}
}

func TestMessagesRejectABadLimit(t *testing.T) {
	s := newScene(t)
	h := s.history()
	_, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(h.conversation), "", core.PageRequest{Limit: 500})
	wantInvalid(t, err, "limit")
}

func TestMessagesRejectABadCursor(t *testing.T) {
	s := newScene(t)
	h := s.history()
	_, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(h.conversation), "", core.PageRequest{Cursor: "abc"})
	wantInvalid(t, err, "cursor")
}

func TestMessagesReportABrokenMessageStore(t *testing.T) {
	s := newScene(t)
	h := s.history()
	s.breaker.broken["messages.ofConversation"] = true
	_, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(h.conversation), "", core.PageRequest{})
	wantError[*core.Unavailable](t, err)
}

func TestMessagesReportABrokenReactionStore(t *testing.T) {
	s := newScene(t)
	h := s.history()
	s.breaker.broken["reactions.ofConversation"] = true
	_, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(h.conversation), "", core.PageRequest{})
	wantError[*core.Unavailable](t, err)
}

func TestMessagesReportABrokenParticipantList(t *testing.T) {
	s := newScene(t)
	h := s.history()
	s.breaker.broken["participants.ofConversation"] = true
	_, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(h.conversation), "", core.PageRequest{})
	wantError[*core.Unavailable](t, err)
}

func TestMessagesStopReadingOnceThePageIsFull(t *testing.T) {
	s := newScene(t)
	h := s.history()
	list, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("ana"), cnvId(h.conversation), "", core.PageRequest{Limit: 1})
	wantNoError(t, err)
	if fmt.Sprint(idsOf(list)) != fmt.Sprint([]string{msgId(h.fifth)}) || list.NextCursor != "5" {
		t.Fatalf("got %v next %q", idsOf(list), list.NextCursor)
	}
}

func TestAReplySentAlsoToTheConversationIsListedInBothPages(t *testing.T) {
	s := newScene(t)
	h := s.history()
	also := s.post(h.conversation, core.Message{Sequence: 6, ThreadRootId: msgId(h.first), AlsoToConversation: true})
	reader := reading.NewReader(s.stores, s.world.Ports)
	conversationPage, err := reader.Messages(s.caller("ana"), cnvId(h.conversation), "", core.PageRequest{})
	wantNoError(t, err)
	threadPage, err := reader.Messages(s.caller("ana"), cnvId(h.conversation), msgId(h.first), core.PageRequest{})
	wantNoError(t, err)
	wantConversation := []string{msgId(also), msgId(h.fifth), msgId(h.third), msgId(h.second), msgId(h.first)}
	wantThread := []string{msgId(also), msgId(h.reply)}
	if fmt.Sprint(idsOf(conversationPage)) != fmt.Sprint(wantConversation) || fmt.Sprint(idsOf(threadPage)) != fmt.Sprint(wantThread) {
		t.Fatalf("conversation %v thread %v", idsOf(conversationPage), idsOf(threadPage))
	}
}

func TestOneMessageIsShownToAParticipant(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	wantNoError(t, reading.NewReactions(s.stores, s.world.Ports).Add(s.caller("bo"), msgId(message), "heart"))
	view, err := reading.NewReader(s.stores, s.world.Ports).Message(s.caller("bo"), msgId(message))
	wantNoError(t, err)
	if msgId(view.Message) != msgId(message) || view.Reactions["heart"] != 1 || fmt.Sprint(view.MyReactions) != "[heart]" || len(view.Blocks) != 1 {
		t.Fatalf("got %+v", view)
	}
}

func TestOneMessageOfAPublicChannelIsShownToAnyMemberOfTheDomain(t *testing.T) {
	s := newScene(t)
	channel := s.conversation(core.Conversation{Kind: "channel", Key: "chan1", Visibility: "public", MessageCount: 1})
	message := s.post(channel, core.Message{Sequence: 1, Body: "news"})
	view, err := reading.NewReader(s.stores, s.world.Ports).Message(s.caller("cy"), msgId(message))
	wantNoError(t, err)
	if view.Message.Body != "news" {
		t.Fatalf("got %+v", view)
	}
}

func TestOneMessageIsHiddenFromOthers(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	reader := reading.NewReader(s.stores, s.world.Ports)
	_, err := reader.Message(s.caller("cy"), msgId(message))
	wantError[*core.NotFound](t, err)
	stranger := s.caller("bo")
	stranger.DomainId = "example.org"
	_, err = reader.Message(stranger, msgId(message))
	wantError[*core.NotFound](t, err)
	_, err = reader.Message(s.caller("bo"), "msg_x")
	wantError[*core.NotFound](t, err)
}

func TestOneMessageReportsABrokenReactionStore(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.breaker.broken["reactions.ofConversation"] = true
	_, err := reading.NewReader(s.stores, s.world.Ports).Message(s.caller("bo"), msgId(message))
	wantError[*core.Unavailable](t, err)
}
