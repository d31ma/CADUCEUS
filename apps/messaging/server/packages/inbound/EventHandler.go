package inbound

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type EventHandler struct {
	stores core.Stores
	ports  core.Ports
}

func NewEventHandler(stores core.Stores, ports core.Ports) EventHandler {
	return EventHandler{stores: stores, ports: ports}
}

func (handler EventHandler) Handle(envelope events.Envelope) error {
	handle, err := handler.handlerFor(envelope.Type)
	if err != nil {
		return err
	}
	return handle(envelope)
}

func (handler EventHandler) handlerFor(eventType string) (func(events.Envelope) error, error) {
	handle, known := handler.table()[eventType]
	if !known {
		return nil, UnhandledEvent{Type: eventType}
	}
	return handle, nil
}

func (handler EventHandler) table() map[string]func(events.Envelope) error {
	types := events.Types{}
	channels := NewChannelEvents(handler.stores.Conversations, handler.ports.Now)
	channelMembers := NewChannelMembers(handler.stores)
	return map[string]func(events.Envelope) error{
		types.MemberUpserted():       NewMemberEvents(handler.stores.Members).Upserted,
		types.ChannelCreated():       channels.Created,
		types.ChannelArchived():      channels.Archived,
		types.ChannelMemberAdded():   channelMembers.Added,
		types.ChannelMemberRemoved(): channelMembers.Removed,
		types.EmailDeliveryChanged(): NewDeliveryEvents(handler.stores.Messages).Changed,
		types.EmailReceived():        NewEmailEvents(handler.stores, handler.ports).Received,
	}
}
