package sending

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"mesail.dev/packages/core"
)

func (s *scene) directWith(t *testing.T, memberCount int) core.Conversation {
	t.Helper()
	conversation := core.Conversation{DomainId: "example.com", Kind: "direct", Visibility: "private", CreatedMonth: "202611"}
	id, _ := s.world.Stores.Conversations.Insert(conversation)
	conversation.Id = id
	conversationId := core.ConversationIds{}.Wire(conversation)
	s.world.Stores.Participants.Insert(core.Participant{DomainId: "example.com", ConversationId: conversationId, Kind: "member", MemberId: "ana", Address: "ana@example.com", Role: "to"})
	for index := 1; index < memberCount; index++ {
		memberId := fmt.Sprintf("m%03d", index)
		s.world.Stores.Participants.Insert(core.Participant{DomainId: "example.com", ConversationId: conversationId, Kind: "member", MemberId: memberId, Address: memberId + "@example.com", Role: "to"})
	}
	return conversation
}

func (s *scene) addTo(conversation core.Conversation, choice core.RecipientChoice) (core.SendOutcome, error) {
	sent := request("c-add", "welcome", choice)
	sent.ConversationId = core.ConversationIds{}.Wire(conversation)
	return s.send(sent)
}

func TestADirectConversationRefusesA101stMember(t *testing.T) {
	s := newScene(t)
	conversation := s.directWith(t, 100)
	before := len(s.participants())
	_, err := s.addTo(conversation, to("cy@example.com", "chat", "member"))
	wantInvalid(t, err, "recipients")
	if len(s.messages()) != 0 || len(s.participants()) != before {
		t.Fatalf("a refused send wrote %d messages and %d participants", len(s.messages()), len(s.participants())-before)
	}
}

func TestADirectConversationTakesItsHundredthMember(t *testing.T) {
	s := newScene(t)
	conversation := s.directWith(t, 99)
	outcome, err := s.addTo(conversation, to("cy@example.com", "chat", "member"))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	payload := created(t, s.stored(t, outcome))
	if payload.ConversationKind != "direct" || len(payload.MemberIds) != 100 {
		t.Fatalf("the event lists %d readers", len(payload.MemberIds))
	}
}

func TestAChannelMessageKeepsTheFirstHundredMentions(t *testing.T) {
	s := newScene(t)
	s.channel(t, false)
	names := []string{}
	for index := range 150 {
		name := fmt.Sprintf("m%03d", index)
		names = append(names, "@"+name)
		s.world.Stores.Members.Save(core.Member{DomainId: "example.com", MemberId: name, LocalPart: name, DisplayName: name, PrimaryAddress: name + "@example.com"})
		s.world.Stores.Participants.Insert(core.Participant{DomainId: "example.com", ConversationId: "cnv_chan1", Kind: "member", MemberId: name, Address: name + "@example.com", Role: "member"})
	}
	outcome := s.mustSend(t, request("c-many", strings.Join(names, " "), to("releases@example.com", "chat", "channel")))
	payload := created(t, s.stored(t, outcome))
	if len(payload.MentionedMemberIds) != 100 || payload.MentionedMemberIds[0] != "m000" || payload.MentionedMemberIds[99] != "m099" || len(payload.MemberIds) != 0 {
		t.Fatalf("the event names %d mentions and %d readers", len(payload.MentionedMemberIds), len(payload.MemberIds))
	}
}

func TestAPostInAChannelOfTenThousandMembersKeepsTheMessageSmall(t *testing.T) {
	s := newScene(t)
	s.channel(t, false)
	for index := range 10000 {
		memberId := fmt.Sprintf("%08d-0000-4000-8000-000000000000", index)
		s.world.Stores.Participants.Insert(core.Participant{DomainId: "example.com", ConversationId: "cnv_chan1", Kind: "member", MemberId: memberId, Address: memberId + "@example.com", Role: "member"})
	}
	outcome := s.mustSend(t, request("c-big", "@bo the build is green", to("releases@example.com", "chat", "channel")))
	message := s.stored(t, outcome)
	encoded, _ := json.Marshal(message)
	conversation, _ := json.Marshal(s.conversations()[0])
	payload := created(t, message)
	if len(encoded) >= 512*1024 || len(conversation) >= 512*1024 || len(payload.MemberIds) != 0 || len(payload.MentionedMemberIds) != 1 {
		t.Fatalf("the message has %d bytes, the conversation %d, %d readers, %d mentions", len(encoded), len(conversation), len(payload.MemberIds), len(payload.MentionedMemberIds))
	}
}
