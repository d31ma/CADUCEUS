package realtime

import "mesail.dev/packages/events"

type DeliveryPlan struct{}

func (plan DeliveryPlan) For(eventType string, payload EventPayload) Delivery {
	return plan.builder(eventType)(eventType, payload)
}

func (plan DeliveryPlan) builder(eventType string) func(string, EventPayload) Delivery {
	builder, known := plan.builders()[eventType]
	if !known {
		return plan.nobody
	}
	return builder
}

func (plan DeliveryPlan) builders() map[string]func(string, EventPayload) Delivery {
	types := events.Types{}
	return map[string]func(string, EventPayload) Delivery{
		types.MessageCreated():    plan.message,
		types.MessageEdited():     plan.message,
		types.MessageDeleted():    plan.message,
		types.ReactionChanged():   plan.reaction,
		types.ReadPositionMoved(): plan.readMoved,
		types.PresenceChanged():   plan.presence,
	}
}

func (plan DeliveryPlan) message(eventType string, payload EventPayload) Delivery {
	event := Event{Type: eventType, ConversationId: payload.ConversationId, MessageId: payload.MessageId, MemberId: payload.AuthorMemberId}
	return plan.toReaders(event, payload)
}

func (plan DeliveryPlan) reaction(eventType string, payload EventPayload) Delivery {
	event := Event{Type: eventType, ConversationId: payload.ConversationId, MessageId: payload.MessageId, MemberId: payload.MemberId, State: payload.Emoji}
	return plan.toReaders(event, payload)
}

func (DeliveryPlan) toReaders(event Event, payload EventPayload) Delivery {
	if payload.FromChannel() {
		return Delivery{Event: event, MemberIds: []string{}, ChannelConversationId: payload.ConversationId}
	}
	return Delivery{Event: event, MemberIds: payload.MemberIds}
}

func (plan DeliveryPlan) readMoved(eventType string, payload EventPayload) Delivery {
	event := Event{Type: plan.readMovedType(), ConversationId: payload.ConversationId, MessageId: payload.MessageId, MemberId: payload.MemberId}
	return Delivery{Event: event, MemberIds: []string{payload.MemberId}}
}

func (DeliveryPlan) presence(eventType string, payload EventPayload) Delivery {
	return Delivery{Event: Event{Type: eventType, MemberId: payload.MemberId, State: payload.State}, ToDomain: true}
}

func (DeliveryPlan) nobody(eventType string, payload EventPayload) Delivery {
	return Delivery{MemberIds: []string{}}
}

func (DeliveryPlan) readMovedType() string {
	return "read.moved"
}
