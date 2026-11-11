package reading_test

import (
	"fmt"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/reading"
)

func (s *scene) sidebar() (core.Conversation, core.Conversation, core.Conversation) {
	direct := s.conversation(core.Conversation{MessageCount: 3, LastMessageAt: "2026-11-10T10:00:00Z"})
	s.join(direct, core.Participant{MemberId: "ana", DisplayName: "Ana", ReadSequence: 1})
	s.join(direct, core.Participant{MemberId: "bo", DisplayName: "Bo"})
	s.join(direct, core.Participant{Kind: "external", Address: "x@ext.org"})
	channel := s.conversation(core.Conversation{Kind: "channel", Key: "chan1", ChannelName: "general", MessageCount: 2, LastMessageAt: "2026-11-10T11:00:00Z"})
	s.join(channel, core.Participant{MemberId: "ana", ReadSequence: 5})
	email := s.conversation(core.Conversation{HasExternal: true, Subject: "Invoice", MessageCount: 1, LastMessageAt: "2026-11-09T09:00:00Z"})
	s.join(email, core.Participant{MemberId: "ana"})
	s.join(email, core.Participant{Kind: "external", Address: "y@ext.org", DisplayName: "Yan"})
	s.world.Stores.Mentions.Insert(core.Mention{DomainId: "example.com", ConversationId: cnvId(direct), MemberId: "ana", Sequence: 2})
	s.world.Stores.Mentions.Insert(core.Mention{DomainId: "example.com", ConversationId: cnvId(direct), MemberId: "ana", Sequence: 1})
	s.world.Stores.Mentions.Insert(core.Mention{DomainId: "example.com", ConversationId: cnvId(channel), MemberId: "ana", Sequence: 3})
	return direct, channel, email
}

func (s *scene) manyConversations(count int) {
	for index := 0; index < count; index++ {
		conversation := s.conversation(core.Conversation{LastMessageAt: fmt.Sprintf("2026-11-10T10:%02d:00Z", index%60)})
		s.join(conversation, core.Participant{MemberId: "ana"})
	}
}

func titlesOf(list core.ConversationList) []string {
	titles := []string{}
	for _, item := range list.Items {
		titles = append(titles, item.ConversationId+"="+item.Title)
	}
	return titles
}

func TestConversationsAreNewestFirstWithTitles(t *testing.T) {
	s := newScene(t)
	direct, channel, email := s.sidebar()
	list, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{})
	wantNoError(t, err)
	got := titlesOf(list)
	want := fmt.Sprint([]string{cnvId(channel) + "=#general", cnvId(direct) + "=Bo, x@ext.org", cnvId(email) + "=Invoice"})
	if fmt.Sprint(got) != want || list.NextCursor != "" {
		t.Fatalf("got %v next %q, want %v", got, list.NextCursor, want)
	}
}

func TestConversationsCountUnreadAndMentions(t *testing.T) {
	s := newScene(t)
	s.sidebar()
	list, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{})
	wantNoError(t, err)
	got := fmt.Sprintf("%d/%d %d/%d %d/%d", list.Items[0].UnreadCount, list.Items[0].MentionCount, list.Items[1].UnreadCount, list.Items[1].MentionCount, list.Items[2].UnreadCount, list.Items[2].MentionCount)
	if got != "0/0 2/1 1/0" {
		t.Fatalf("got unread/mentions %s", got)
	}
}

func TestConversationsShowOnlyTheCallersDomain(t *testing.T) {
	s := newScene(t)
	s.sidebar()
	other := s.conversation(core.Conversation{DomainId: "example.org"})
	s.join(other, core.Participant{MemberId: "ana"})
	list, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{})
	wantNoError(t, err)
	if len(list.Items) != 3 {
		t.Fatalf("want 3 conversations, got %d", len(list.Items))
	}
}

func TestConversationsSkipMissingConversations(t *testing.T) {
	s := newScene(t)
	s.world.Stores.Participants.Insert(core.Participant{DomainId: "example.com", ConversationId: "cnv_999999999999", MemberId: "ana"})
	s.world.Stores.Participants.Insert(core.Participant{DomainId: "example.com", ConversationId: "broken", MemberId: "ana"})
	list, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{})
	wantNoError(t, err)
	if len(list.Items) != 0 {
		t.Fatalf("want none, got %+v", list.Items)
	}
}

func TestConversationsArePaged(t *testing.T) {
	s := newScene(t)
	direct, channel, email := s.sidebar()
	reader := reading.NewReader(s.stores, s.world.Ports)
	first, err := reader.Conversations(s.caller("ana"), core.PageRequest{Limit: 2})
	wantNoError(t, err)
	second, err := reader.Conversations(s.caller("ana"), core.PageRequest{Limit: 2, Cursor: first.NextCursor})
	wantNoError(t, err)
	got := fmt.Sprint(first.Items[0].ConversationId, first.Items[1].ConversationId, first.NextCursor, second.Items[0].ConversationId, len(second.Items), second.NextCursor == "")
	want := fmt.Sprint(cnvId(channel), cnvId(direct), "2", cnvId(email), 1, true)
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestConversationsCursorPastTheEndIsEmpty(t *testing.T) {
	s := newScene(t)
	s.sidebar()
	list, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{Cursor: "10"})
	wantNoError(t, err)
	if len(list.Items) != 0 || list.NextCursor != "" {
		t.Fatalf("got %+v", list)
	}
}

func TestConversationsDefaultLimitIsFifty(t *testing.T) {
	s := newScene(t)
	s.manyConversations(51)
	list, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{})
	wantNoError(t, err)
	if len(list.Items) != 50 || list.NextCursor != "50" {
		t.Fatalf("got %d items, next %q", len(list.Items), list.NextCursor)
	}
}

func TestConversationsRejectALimitAboveOneHundred(t *testing.T) {
	s := newScene(t)
	_, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{Limit: 101})
	wantInvalid(t, err, "limit")
}

func TestConversationsRejectANegativeLimit(t *testing.T) {
	s := newScene(t)
	_, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{Limit: -1})
	wantInvalid(t, err, "limit")
}

func TestConversationsRejectATextCursor(t *testing.T) {
	s := newScene(t)
	_, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{Cursor: "x"})
	wantInvalid(t, err, "cursor")
}

func TestConversationsRejectANegativeCursor(t *testing.T) {
	s := newScene(t)
	_, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{Cursor: "-3"})
	wantInvalid(t, err, "cursor")
}

func TestConversationsReportABrokenParticipantStore(t *testing.T) {
	s := newScene(t)
	s.breaker.broken["participants.ofMember"] = true
	_, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{})
	wantError[*core.Unavailable](t, err)
}

func TestConversationsReportABrokenMentionStore(t *testing.T) {
	s := newScene(t)
	s.breaker.broken["mentions.ofMember"] = true
	_, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{})
	wantError[*core.Unavailable](t, err)
}

func TestConversationsReportABrokenConversationStore(t *testing.T) {
	s := newScene(t)
	s.sidebar()
	s.breaker.broken["conversations.find"] = true
	_, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{})
	wantError[*core.Unavailable](t, err)
}

func TestConversationsReportABrokenTitleLookup(t *testing.T) {
	s := newScene(t)
	s.sidebar()
	s.breaker.broken["participants.ofConversation"] = true
	_, err := reading.NewReader(s.stores, s.world.Ports).Conversations(s.caller("ana"), core.PageRequest{})
	wantError[*core.Unavailable](t, err)
}
