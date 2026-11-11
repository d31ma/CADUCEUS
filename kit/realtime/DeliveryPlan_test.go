package realtime

import (
	"reflect"
	"testing"
)

func TestMessageEventsGoToTheReadersWithTheAuthor(t *testing.T) {
	payload := EventPayload{MessageId: "msg_1", ConversationId: "cnv_1", AuthorMemberId: "ana", MemberIds: []string{"ana", "bo"}}
	delivery := DeliveryPlan{}.For("message.created", payload)
	want := Delivery{Event: Event{Type: "message.created", ConversationId: "cnv_1", MessageId: "msg_1", MemberId: "ana"}, MemberIds: []string{"ana", "bo"}}
	if !reflect.DeepEqual(delivery, want) {
		t.Fatalf("got %+v", delivery)
	}
}

func TestEditedAndDeletedMessagesCarryNoMember(t *testing.T) {
	payload := EventPayload{MessageId: "msg_1", ConversationId: "cnv_1", MemberIds: []string{"bo"}}
	edited := DeliveryPlan{}.For("message.edited", payload)
	deleted := DeliveryPlan{}.For("message.deleted", payload)
	if edited.Event.Type != "message.edited" || edited.Event.MemberId != "" || deleted.Event.Type != "message.deleted" || !reflect.DeepEqual(deleted.MemberIds, []string{"bo"}) {
		t.Fatalf("got %+v %+v", edited, deleted)
	}
}

func TestReactionEventsCarryTheEmojiAsState(t *testing.T) {
	payload := EventPayload{MessageId: "msg_1", ConversationId: "cnv_1", MemberId: "bo", Emoji: "+1", MemberIds: []string{"ana", "bo"}}
	delivery := DeliveryPlan{}.For("reaction.changed", payload)
	want := Delivery{Event: Event{Type: "reaction.changed", ConversationId: "cnv_1", MessageId: "msg_1", MemberId: "bo", State: "+1"}, MemberIds: []string{"ana", "bo"}}
	if !reflect.DeepEqual(delivery, want) {
		t.Fatalf("got %+v", delivery)
	}
}

func TestReadPositionGoesOnlyToTheReader(t *testing.T) {
	payload := EventPayload{MessageId: "msg_1", ConversationId: "cnv_1", MemberId: "ana"}
	delivery := DeliveryPlan{}.For("read-position.moved", payload)
	want := Delivery{Event: Event{Type: "read.moved", ConversationId: "cnv_1", MessageId: "msg_1", MemberId: "ana"}, MemberIds: []string{"ana"}}
	if !reflect.DeepEqual(delivery, want) {
		t.Fatalf("got %+v", delivery)
	}
}

func TestPresenceChangesGoToTheWholeDomain(t *testing.T) {
	delivery := DeliveryPlan{}.For("presence.changed", EventPayload{MemberId: "ana", State: "idle"})
	want := Delivery{Event: Event{Type: "presence.changed", MemberId: "ana", State: "idle"}, ToDomain: true}
	if !reflect.DeepEqual(delivery, want) {
		t.Fatalf("got %+v", delivery)
	}
}

func TestChannelEventsAreDeliveredToNobody(t *testing.T) {
	delivery := DeliveryPlan{}.For("channel.created", EventPayload{ConversationId: "cnv_1", CreatedBy: "ana"})
	if delivery.ToDomain || len(delivery.MemberIds) != 0 {
		t.Fatalf("got %+v", delivery)
	}
}
