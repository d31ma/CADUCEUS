package realtime

import (
	"encoding/json"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

type EventDesk struct {
	Gate    oidc.Gate
	Seen    SeenBook
	Members MemberBook
	FanOut  FanOut
}

func (desk EventDesk) Receive(authorization string, envelope events.Envelope) (events.Receipt, error) {
	caller, err := CallerGate{Gate: desk.Gate}.Caller(authorization)
	if err != nil {
		return events.Receipt{}, err
	}
	err = desk.sameDomain(caller, envelope)
	if err != nil {
		return events.Receipt{}, err
	}
	return events.NewReceiver(events.Services{}.Realtime(), desk.Seen, desk).Receive(caller, envelope)
}

func (desk EventDesk) Handle(envelope events.Envelope) error {
	payload, err := desk.payload(envelope)
	if err != nil {
		return err
	}
	err = desk.membership(envelope.Type)(envelope.DomainId, payload)
	if err != nil {
		return err
	}
	return desk.deliver(envelope.DomainId, DeliveryPlan{}.For(envelope.Type, payload))
}

func (EventDesk) sameDomain(caller oidc.Caller, envelope events.Envelope) error {
	if caller.DomainId != envelope.DomainId {
		return Missing{What: "domain"}
	}
	return nil
}

func (EventDesk) payload(envelope events.Envelope) (EventPayload, error) {
	payload := EventPayload{MemberIds: []string{}}
	err := json.Unmarshal(envelope.Payload, &payload)
	if err != nil {
		return payload, events.InvalidEnvelope{Reason: "the payload is not a JSON object of the expected shape"}
	}
	return payload, nil
}

func (desk EventDesk) membership(eventType string) func(string, EventPayload) error {
	change, known := desk.membershipChanges()[eventType]
	if !known {
		return desk.unchanged
	}
	return change
}

func (desk EventDesk) membershipChanges() map[string]func(string, EventPayload) error {
	types := events.Types{}
	return map[string]func(string, EventPayload) error{
		types.ChannelCreated():       desk.channelCreated,
		types.ChannelMemberAdded():   desk.memberAdded,
		types.ChannelMemberRemoved(): desk.memberRemoved,
		types.MessageCreated():       desk.messageMembers,
		types.MessageEdited():        desk.messageMembers,
		types.MessageDeleted():       desk.messageMembers,
	}
}

func (desk EventDesk) channelCreated(domainId string, payload EventPayload) error {
	if payload.CreatedBy == "" {
		return nil
	}
	return desk.Members.Add(domainId, payload.ConversationId, payload.CreatedBy)
}

func (desk EventDesk) memberAdded(domainId string, payload EventPayload) error {
	return desk.Members.Add(domainId, payload.ConversationId, payload.MemberId)
}

func (desk EventDesk) memberRemoved(domainId string, payload EventPayload) error {
	return desk.Members.Remove(domainId, payload.ConversationId, payload.MemberId)
}

func (desk EventDesk) messageMembers(domainId string, payload EventPayload) error {
	return desk.Members.Replace(domainId, payload.ConversationId, payload.MemberIds)
}

func (EventDesk) unchanged(domainId string, payload EventPayload) error {
	return nil
}

func (desk EventDesk) deliver(domainId string, delivery Delivery) error {
	switch {
	case delivery.ToDomain:
		return desk.FanOut.ToDomain(domainId, delivery.Event)
	case len(delivery.MemberIds) > 0:
		return desk.FanOut.ToMembers(domainId, delivery.MemberIds, delivery.Event)
	}
	return nil
}
