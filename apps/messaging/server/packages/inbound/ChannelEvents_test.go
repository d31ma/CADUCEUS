package inbound

import (
	"errors"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

func TestChannelCreatedMakesTheChannelConversationOnce(t *testing.T) {
	scene := NewScene(t)
	scene.Channel("example.com", "chan1")
	scene.Channel("example.com", "chan1")
	conversations := scene.Conversations()
	if len(conversations) != 1 {
		t.Fatalf("want one conversation, got %+v", conversations)
	}
	conversation := conversations[0]
	if (core.ConversationIds{}).Wire(conversation) != "cnv_chan1" || conversation.Kind != "channel" || conversation.ChannelName != "support" {
		t.Fatalf("got %+v", conversation)
	}
	if conversation.EmailAddress != "support@example.com" || conversation.Visibility != "public" || conversation.CreatedMonth != "202611" || conversation.CreatedAt != "2026-11-10T12:00:00Z" {
		t.Fatalf("got %+v", conversation)
	}
}

func TestChannelCreatedWithABadConversationIdIsRefused(t *testing.T) {
	scene := NewScene(t)
	err := scene.Send(events.Types{}.ChannelCreated(), "example.com", events.ChannelCreated{ChannelId: "chan1", ConversationId: "chan1"})
	if !errors.As(err, new(*core.NotFound)) {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestChannelCreatedReportsStoreFailures(t *testing.T) {
	assertChannelCreatedFails(t, 1)
	assertChannelCreatedFails(t, 2)
}

func assertChannelCreatedFails(t *testing.T, call int) {
	t.Helper()
	scene := NewScene(t)
	scene.FailAt(call)
	err := scene.Send(events.Types{}.ChannelCreated(), "example.com", events.ChannelCreated{ChannelId: "chan1", ConversationId: "cnv_chan1"})
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("call %d: want Unavailable, got %v", call, err)
	}
}

func TestChannelMemberAddedBeforeTheChannelAsksForARetry(t *testing.T) {
	scene := NewScene(t)
	added := events.ChannelMemberAdded{ChannelId: "chan1", ConversationId: "cnv_chan1", MemberId: "bo", JoinedAt: "2026-11-10T12:00:00Z"}
	err := scene.Send(events.Types{}.ChannelMemberAdded(), "example.com", added)
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want Unavailable, got %v", err)
	}
	scene.Must(events.Types{}.MemberUpserted(), "example.com", events.MemberUpserted{MemberId: "bo", DisplayName: "Bo", PrimaryAddress: "bo@example.com", State: "active"})
	scene.Channel("example.com", "chan1")
	scene.Must(events.Types{}.ChannelMemberAdded(), "example.com", added)
	scene.Must(events.Types{}.ChannelMemberAdded(), "example.com", added)
	participants := scene.Participants()
	if len(participants) != 1 {
		t.Fatalf("want one participant, got %+v", participants)
	}
	want := core.Participant{Id: participants[0].Id, DomainId: "example.com", ConversationId: "cnv_chan1", Kind: "member", MemberId: "bo", Address: "bo@example.com", DisplayName: "Bo", Role: "member"}
	if participants[0].Id == "" || participants[0].Address != want.Address || participants[0].DisplayName != want.DisplayName || participants[0].Role != want.Role || participants[0].DomainId != want.DomainId || participants[0].ConversationId != want.ConversationId {
		t.Fatalf("got %+v", participants[0])
	}
}

func TestChannelMemberAddedForAnUnknownMemberStillJoins(t *testing.T) {
	scene := NewScene(t)
	scene.Channel("example.com", "chan1")
	scene.Must(events.Types{}.ChannelMemberAdded(), "example.com", events.ChannelMemberAdded{ChannelId: "chan1", ConversationId: "cnv_chan1", MemberId: "cy"})
	participants := scene.Participants()
	if len(participants) != 1 || participants[0].MemberId != "cy" || participants[0].DomainId != "example.com" {
		t.Fatalf("got %+v", participants)
	}
}

func TestChannelMemberAddedReportsStoreFailures(t *testing.T) {
	assertMemberAddedFails(t, 1)
	assertMemberAddedFails(t, 2)
	assertMemberAddedFails(t, 3)
	assertMemberAddedFails(t, 4)
}

func assertMemberAddedFails(t *testing.T, call int) {
	t.Helper()
	scene := NewScene(t)
	scene.Channel("example.com", "chan1")
	scene.FailAt(call)
	err := scene.Send(events.Types{}.ChannelMemberAdded(), "example.com", events.ChannelMemberAdded{ChannelId: "chan1", ConversationId: "cnv_chan1", MemberId: "bo"})
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("call %d: want Unavailable, got %v", call, err)
	}
}

func TestChannelMemberRemovedDeletesOnlyThatMember(t *testing.T) {
	scene := NewScene(t)
	scene.Channel("example.com", "chan1")
	scene.Must(events.Types{}.ChannelMemberAdded(), "example.com", events.ChannelMemberAdded{ChannelId: "chan1", ConversationId: "cnv_chan1", MemberId: "bo"})
	scene.Must(events.Types{}.ChannelMemberAdded(), "example.com", events.ChannelMemberAdded{ChannelId: "chan1", ConversationId: "cnv_chan1", MemberId: "ana"})
	removed := events.ChannelMemberRemoved{ChannelId: "chan1", ConversationId: "cnv_chan1", MemberId: "bo"}
	scene.Must(events.Types{}.ChannelMemberRemoved(), "example.com", removed)
	scene.Must(events.Types{}.ChannelMemberRemoved(), "example.com", removed)
	participants := scene.Participants()
	if len(participants) != 1 || participants[0].MemberId != "ana" {
		t.Fatalf("got %+v", participants)
	}
}

func TestChannelMemberRemovedReportsStoreFailures(t *testing.T) {
	assertMemberRemovedFails(t, 1)
	assertMemberRemovedFails(t, 2)
}

func assertMemberRemovedFails(t *testing.T, call int) {
	t.Helper()
	scene := NewScene(t)
	scene.Channel("example.com", "chan1")
	scene.Must(events.Types{}.ChannelMemberAdded(), "example.com", events.ChannelMemberAdded{ChannelId: "chan1", ConversationId: "cnv_chan1", MemberId: "bo"})
	scene.FailAt(call)
	err := scene.Send(events.Types{}.ChannelMemberRemoved(), "example.com", events.ChannelMemberRemoved{ChannelId: "chan1", ConversationId: "cnv_chan1", MemberId: "bo"})
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("call %d: want Unavailable, got %v", call, err)
	}
}

func TestChannelArchivedMarksTheConversationAgainAndAgain(t *testing.T) {
	scene := NewScene(t)
	archived := events.ChannelArchived{ChannelId: "chan1", ConversationId: "cnv_chan1"}
	err := scene.Send(events.Types{}.ChannelArchived(), "example.com", archived)
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want Unavailable before the channel exists, got %v", err)
	}
	scene.Channel("example.com", "chan1")
	scene.Must(events.Types{}.ChannelArchived(), "example.com", archived)
	scene.Must(events.Types{}.ChannelArchived(), "example.com", archived)
	conversations := scene.Conversations()
	if len(conversations) != 1 || !conversations[0].Archived {
		t.Fatalf("got %+v", conversations)
	}
}

func TestChannelArchivedReportsStoreFailures(t *testing.T) {
	assertArchivedFails(t, 1)
	assertArchivedFails(t, 2)
}

func assertArchivedFails(t *testing.T, call int) {
	t.Helper()
	scene := NewScene(t)
	scene.Channel("example.com", "chan1")
	scene.FailAt(call)
	err := scene.Send(events.Types{}.ChannelArchived(), "example.com", events.ChannelArchived{ChannelId: "chan1", ConversationId: "cnv_chan1"})
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("call %d: want Unavailable, got %v", call, err)
	}
}

func TestChannelEventsStayInTheirDomain(t *testing.T) {
	scene := NewScene(t)
	scene.Channel("example.com", "chan1")
	err := scene.Send(events.Types{}.ChannelArchived(), "example.org", events.ChannelArchived{ChannelId: "chan1", ConversationId: "cnv_chan1"})
	if !errors.As(err, new(*core.Unavailable)) || scene.Conversations()[0].Archived {
		t.Fatalf("another domain archived the channel: %v", err)
	}
}
