package channels

import (
	"crypto/rand"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type Events struct {
	settings Settings
	now      func() time.Time
}

func NewEvents(settings Settings, now func() time.Time) Events {
	return Events{settings: settings, now: now}
}

func (service Events) Announce(document ChannelDocument) (ChannelDocument, error) {
	document.ConversationId = WireIds{}.ConversationId(document.Id)
	payload := events.ChannelCreated{
		ChannelId:      WireIds{}.ChannelId(document.Id),
		ConversationId: document.ConversationId,
		Name:           document.Name,
		Topic:          document.Topic,
		Visibility:     document.Visibility,
		EmailAddress:   document.EmailAddress,
		CreatedBy:      document.CreatedBy,
	}
	fields, err := service.stage(service.channelFields(document), events.Types{}.ChannelCreated(), document.DomainId, payload)
	return service.withChannelFields(document, fields), err
}

func (service Events) Archived(document ChannelDocument) (ChannelDocument, error) {
	payload := events.ChannelArchived{ChannelId: WireIds{}.ChannelId(document.Id), ConversationId: document.ConversationId}
	fields, err := service.stage(service.channelFields(document), events.Types{}.ChannelArchived(), document.DomainId, payload)
	return service.withChannelFields(document, fields), err
}

func (service Events) MemberAdded(channel ChannelDocument, membership MembershipDocument) (MembershipDocument, error) {
	payload := events.ChannelMemberAdded{
		ChannelId:      WireIds{}.ChannelId(channel.Id),
		ConversationId: channel.ConversationId,
		MemberId:       membership.MemberId,
		JoinedAt:       membership.JoinedAt,
	}
	fields, err := service.stage(service.membershipFields(membership), events.Types{}.ChannelMemberAdded(), channel.DomainId, payload)
	return service.withMembershipFields(membership, fields), err
}

func (service Events) MemberRemoved(channel ChannelDocument, membership MembershipDocument) (MembershipDocument, error) {
	payload := events.ChannelMemberRemoved{ChannelId: WireIds{}.ChannelId(channel.Id), ConversationId: channel.ConversationId, MemberId: membership.MemberId}
	fields, err := service.stage(service.membershipFields(membership), events.Types{}.ChannelMemberRemoved(), channel.DomainId, payload)
	return service.withMembershipFields(membership, fields), err
}

func (service Events) Ready() error {
	if service.settings.OutboxError != nil {
		return &Unavailable{Message: "the outbox settings are missing", Cause: service.settings.OutboxError}
	}
	return nil
}

func (service Events) stage(fields outbox.Fields, eventType string, domainId string, payload any) (outbox.Fields, error) {
	err := service.Ready()
	if err != nil {
		return fields, err
	}
	envelope, err := events.NewEnvelopeFactory(service.producer(), service.now, rand.Reader).New(eventType, domainId, payload)
	if err != nil {
		return fields, err
	}
	return outbox.NewOutbox(service.settings.Outbox).Stage(fields, envelope)
}

func (Events) channelFields(document ChannelDocument) outbox.Fields {
	return outbox.Fields{PendingEvents: document.PendingEvents, DeadEvents: document.DeadEvents, OutboxPending: document.OutboxPending, OutboxDead: document.OutboxDead}
}

func (Events) withChannelFields(document ChannelDocument, fields outbox.Fields) ChannelDocument {
	patch := OutboxPatch{}.From(fields)
	document.PendingEvents, document.DeadEvents = patch.PendingEvents, patch.DeadEvents
	document.OutboxPending, document.OutboxDead = patch.OutboxPending, patch.OutboxDead
	document.OutboxWaiting, document.OutboxHasDead = patch.OutboxWaiting, patch.OutboxHasDead
	return document
}

func (Events) membershipFields(membership MembershipDocument) outbox.Fields {
	return outbox.Fields{PendingEvents: membership.PendingEvents, DeadEvents: membership.DeadEvents, OutboxPending: membership.OutboxPending, OutboxDead: membership.OutboxDead}
}

func (Events) withMembershipFields(membership MembershipDocument, fields outbox.Fields) MembershipDocument {
	patch := OutboxPatch{}.From(fields)
	membership.PendingEvents, membership.DeadEvents = patch.PendingEvents, patch.DeadEvents
	membership.OutboxPending, membership.OutboxDead = patch.OutboxPending, patch.OutboxDead
	membership.OutboxWaiting, membership.OutboxHasDead = patch.OutboxWaiting, patch.OutboxHasDead
	return membership
}

func (Events) producer() string {
	return "channels"
}
