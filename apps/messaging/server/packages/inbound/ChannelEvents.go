package inbound

import (
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type ChannelEvents struct {
	conversations core.ConversationStore
	now           func() time.Time
}

func NewChannelEvents(conversations core.ConversationStore, now func() time.Time) ChannelEvents {
	return ChannelEvents{conversations: conversations, now: now}
}

func (channelEvents ChannelEvents) Created(envelope events.Envelope) error {
	payload := events.ChannelCreated{}
	err := events.PayloadReader{}.Read(envelope, &payload)
	if err != nil {
		return err
	}
	key, err := core.ConversationIds{}.Parse(payload.ConversationId)
	if err != nil {
		return err
	}
	_, found, err := channelEvents.conversations.ByChannel(envelope.DomainId, payload.ChannelId)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	err = channelEvents.insertUnlessFound(channelEvents.conversation(envelope.DomainId, key, payload), found)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	return nil
}

func (channelEvents ChannelEvents) Archived(envelope events.Envelope) error {
	payload := events.ChannelArchived{}
	err := events.PayloadReader{}.Read(envelope, &payload)
	if err != nil {
		return err
	}
	conversation, err := NewChannelConversations(channelEvents.conversations).Existing(envelope.DomainId, payload.ChannelId)
	if err != nil {
		return err
	}
	conversation.Archived = true
	err = channelEvents.conversations.Save(conversation)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	return nil
}

func (channelEvents ChannelEvents) insertUnlessFound(conversation core.Conversation, found bool) error {
	if found {
		return nil
	}
	_, err := channelEvents.conversations.Insert(conversation)
	return err
}

func (channelEvents ChannelEvents) conversation(domainId string, key string, payload events.ChannelCreated) core.Conversation {
	now := channelEvents.now()
	return core.Conversation{
		DomainId:     domainId,
		Key:          key,
		Kind:         core.ConversationKinds{}.Channel(),
		ChannelId:    payload.ChannelId,
		ChannelName:  payload.Name,
		Visibility:   payload.Visibility,
		EmailAddress: payload.EmailAddress,
		CreatedMonth: core.MessageIds{}.Month(now),
		CreatedAt:    Timestamps{}.Of(now),
	}
}
