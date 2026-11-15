package realtime

import (
	"errors"
	"reflect"
	"testing"

	"mesail.dev/packages/events"
)

func TestReceiveRefusesAMissingToken(t *testing.T) {
	desk := presenceEventDesk(newMemoryStorage(), newRecordingGateway())
	_, err := desk.Receive("", presenceEnvelope("evt_1", "channel.archived", "example.com", events.ChannelArchived{}))
	if !errors.As(err, &Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestReceiveAnswersMissingForAServiceTokenOfAnotherDomain(t *testing.T) {
	desk := presenceEventDesk(newMemoryStorage(), newRecordingGateway())
	_, err := desk.Receive("Bearer elsewhere", presenceEnvelope("evt_1", "channel.archived", "example.com", events.ChannelArchived{}))
	if !errors.As(err, &Missing{}) {
		t.Fatalf("got %v", err)
	}
}

func TestReceiveRefusesAMemberToken(t *testing.T) {
	desk := presenceEventDesk(newMemoryStorage(), newRecordingGateway())
	_, err := desk.Receive("Bearer ana", presenceEnvelope("evt_1", "channel.archived", "example.com", events.ChannelArchived{}))
	if !errors.Is(err, events.NotService{}) {
		t.Fatalf("got %v", err)
	}
}

func TestReceiveRefusesAnEventRealtimeDoesNotConsume(t *testing.T) {
	desk := presenceEventDesk(newMemoryStorage(), newRecordingGateway())
	_, err := desk.Receive("Bearer service", presenceEnvelope("evt_1", "member.upserted", "example.com", events.MemberUpserted{}))
	if !errors.As(err, &events.NotSubscribed{}) {
		t.Fatalf("got %v", err)
	}
}

func TestReceiveAppliesAnEventOnceAndAnswersDuplicateAfter(t *testing.T) {
	memory := newMemoryStorage()
	desk := presenceEventDesk(memory, newRecordingGateway())
	envelope := presenceEnvelope("evt_1", "channel.member.added", "example.com", events.ChannelMemberAdded{ChannelId: "chn_1", ConversationId: "cnv_1", MemberId: "bo"})
	first, err := desk.Receive("Bearer service", envelope)
	desk.Members.Remove("example.com", "cnv_1", "bo")
	second, again := desk.Receive("Bearer service", envelope)
	members, _ := desk.Members.Members("example.com", "cnv_1")
	if err != nil || again != nil || first.Status != "applied" || second.Status != "duplicate" || len(members) != 0 {
		t.Fatalf("got %+v %+v %v %v %v", first, second, members, err, again)
	}
}

func handled(t *testing.T, desk EventDesk, eventType string, payload any) {
	t.Helper()
	if err := desk.Handle(presenceEnvelope("evt_x", eventType, "example.com", payload)); err != nil {
		t.Fatal(err)
	}
}

func TestHandleAddsTheCreatorOfANewChannel(t *testing.T) {
	desk := presenceEventDesk(newMemoryStorage(), newRecordingGateway())
	handled(t, desk, "channel.created", events.ChannelCreated{ChannelId: "chn_1", ConversationId: "cnv_1", CreatedBy: "ana"})
	members, _ := desk.Members.Members("example.com", "cnv_1")
	if !reflect.DeepEqual(members, []string{"ana"}) {
		t.Fatalf("got %v", members)
	}
}

func TestHandleSkipsAChannelWithoutCreator(t *testing.T) {
	memory := newMemoryStorage()
	desk := presenceEventDesk(memory, newRecordingGateway())
	handled(t, desk, "channel.created", events.ChannelCreated{ChannelId: "chn_1", ConversationId: "cnv_1"})
	handled(t, desk, "channel.archived", events.ChannelArchived{ChannelId: "chn_1", ConversationId: "cnv_1"})
	if memory.writes != 0 {
		t.Fatalf("got %d writes", memory.writes)
	}
}

func TestHandleFollowsChannelMembership(t *testing.T) {
	desk := presenceEventDesk(newMemoryStorage(), newRecordingGateway())
	handled(t, desk, "channel.member.added", events.ChannelMemberAdded{ConversationId: "cnv_1", MemberId: "ana"})
	handled(t, desk, "channel.member.added", events.ChannelMemberAdded{ConversationId: "cnv_1", MemberId: "bo"})
	handled(t, desk, "channel.member.removed", events.ChannelMemberRemoved{ConversationId: "cnv_1", MemberId: "ana"})
	members, _ := desk.Members.Members("example.com", "cnv_1")
	if !reflect.DeepEqual(members, []string{"bo"}) {
		t.Fatalf("got %v", members)
	}
}

func TestHandleTakesTheReadersOfAMessageAsMembers(t *testing.T) {
	desk := presenceEventDesk(newMemoryStorage(), newRecordingGateway())
	handled(t, desk, "message.created", events.MessageCreated{MessageId: "msg_1", ConversationId: "cnv_1", AuthorMemberId: "ana", MemberIds: []string{"bo", "ana"}})
	handled(t, desk, "reaction.changed", events.ReactionChanged{MessageId: "msg_1", ConversationId: "cnv_1", MemberId: "bo", Emoji: "+1", MemberIds: []string{"ana", "bo"}})
	handled(t, desk, "read-position.moved", events.ReadPositionMoved{MessageId: "msg_1", ConversationId: "cnv_1", MemberId: "bo"})
	handled(t, desk, "presence.changed", events.PresenceChanged{MemberId: "bo", State: "idle"})
	members, _ := desk.Members.Members("example.com", "cnv_1")
	if !reflect.DeepEqual(members, []string{"ana", "bo"}) {
		t.Fatalf("got %v", members)
	}
}

func TestHandleRefusesAPayloadThatIsNotAnObject(t *testing.T) {
	desk := presenceEventDesk(newMemoryStorage(), newRecordingGateway())
	envelope := presenceEnvelope("evt_1", "message.created", "example.com", "text")
	if err := desk.Handle(envelope); !errors.As(err, &events.InvalidEnvelope{}) {
		t.Fatalf("got %v", err)
	}
}

func TestHandlePassesAStorageFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	memory.failAll = true
	desk := presenceEventDesk(memory, newRecordingGateway())
	if err := desk.Handle(presenceEnvelope("evt_1", "channel.member.added", "example.com", events.ChannelMemberAdded{ConversationId: "cnv_1", MemberId: "ana"})); err == nil {
		t.Fatal("a failing storage was not reported")
	}
}

func TestReceiveTellsOnlyTheListedLinkedDevicesThatAnEnvelopeWaits(t *testing.T) {
	memory := newMemoryStorage()
	gateway := newRecordingGateway()
	desk := presenceEventDesk(memory, gateway)
	desk.FanOut.Connections.Add(deviceConnection("example.com", "ana", "a-listed", "dev_a1", true))
	desk.FanOut.Connections.Add(deviceConnection("example.com", "ana", "a-no-device", "", false))
	desk.FanOut.Connections.Add(deviceConnection("example.com", "ben", "b-other", "dev_b1", true))
	stored := events.EnvelopeStored{ToPersonId: "ana", ToDeviceIds: []string{"dev_a1"}, Kind: "message", EnvelopeId: "env_secret"}
	receipt, err := desk.Receive("Bearer service", presenceEnvelope("evt_w", "envelope.stored", "example.com", stored))
	want := []Event{{Type: "devicebox.waiting", State: "message"}}
	if err != nil || receipt.Status != "applied" || !reflect.DeepEqual(gateway.eventsOf("a-listed"), want) || len(gateway.eventsOf("a-no-device")) != 0 || len(gateway.eventsOf("b-other")) != 0 {
		t.Fatalf("got %+v %v %+v", receipt, err, gateway.pushes)
	}
}
