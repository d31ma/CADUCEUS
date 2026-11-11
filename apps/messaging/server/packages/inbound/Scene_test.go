package inbound

import (
	"crypto/rand"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/fakes"
	"mesail.dev/packages/outbox"
)

type Scene struct {
	t       *testing.T
	world   *fakes.FakeWorld
	memory  core.Stores
	handler EventHandler
}

func NewScene(t *testing.T) *Scene {
	t.Helper()
	world := fakes.NewFakeWorld()
	world.Directory.Member("ana@example.com", "ana", "Ana")
	world.Directory.Member("bo@example.com", "bo", "Bo")
	world.Directory.Channel("support@example.com", "chan1")
	return &Scene{t: t, world: world, memory: world.Stores, handler: NewEventHandler(world.Stores, world.Ports)}
}

func (scene *Scene) FailAt(call int) *Gate {
	stores, gate := FailingAt(scene.memory, call)
	scene.handler = NewEventHandler(stores, scene.world.Ports)
	return gate
}

func (scene *Scene) Send(eventType string, domainId string, payload any) error {
	scene.t.Helper()
	producer, err := events.Catalog{}.ProducerOf(eventType)
	if err != nil {
		scene.t.Fatalf("producer: %v", err)
	}
	envelope, err := events.NewEnvelopeFactory(producer, scene.world.Clock.Now, rand.Reader).New(eventType, domainId, payload)
	if err != nil {
		scene.t.Fatalf("envelope: %v", err)
	}
	return scene.handler.Handle(envelope)
}

func (scene *Scene) Must(eventType string, domainId string, payload any) {
	scene.t.Helper()
	err := scene.Send(eventType, domainId, payload)
	if err != nil {
		scene.t.Fatalf("%s: %v", eventType, err)
	}
}

func (scene *Scene) Members() []core.Member {
	return scene.memory.Members.(*core.MemoryMembers).All()
}

func (scene *Scene) Conversations() []core.Conversation {
	return scene.memory.Conversations.(*core.MemoryConversations).All()
}

func (scene *Scene) Participants() []core.Participant {
	return scene.memory.Participants.(*core.MemoryParticipants).All()
}

func (scene *Scene) Messages() []core.Message {
	return scene.memory.Messages.(*core.MemoryMessages).All()
}

func (scene *Scene) ThreadKeys() []core.ThreadKey {
	return scene.memory.ThreadKeys.(*core.MemoryThreadKeys).All()
}

func (scene *Scene) Channel(domainId string, channelId string) {
	scene.t.Helper()
	scene.Must(events.Types{}.ChannelCreated(), domainId, events.ChannelCreated{ChannelId: channelId, ConversationId: "cnv_" + channelId, Name: "support", Visibility: "public", EmailAddress: "support@" + domainId, CreatedBy: "ana"})
}

func (scene *Scene) Created(message core.Message) events.MessageCreated {
	scene.t.Helper()
	payload := events.MessageCreated{}
	_, err := outbox.Published{}.One(message.OutboxFields.Fields(), events.Types{}.MessageCreated(), &payload)
	if err != nil {
		scene.t.Fatalf("message.created: %v", err)
	}
	return payload
}

func (scene *Scene) SentMessage(domainId string) (core.Conversation, core.Message) {
	scene.t.Helper()
	conversation := core.Conversation{DomainId: domainId, Kind: "direct", HasExternal: true, Subject: "Plans", MessageCount: 1}
	conversation.Id, _ = scene.memory.Conversations.Insert(conversation)
	conversationId := core.ConversationIds{}.Wire(conversation)
	scene.memory.Participants.Insert(core.Participant{DomainId: domainId, ConversationId: conversationId, Kind: "member", MemberId: "ana", Address: "ana@" + domainId, Role: "to"})
	scene.memory.Participants.Insert(core.Participant{DomainId: domainId, ConversationId: conversationId, Kind: "external", Address: "carol@outside.test", Role: "to"})
	message := core.Message{DomainId: domainId, ConversationId: conversationId, Month: "202611", Sequence: 1, AuthorKind: "member", AuthorMemberId: "ana", EventsStaged: true}
	message.Id, _ = scene.memory.Messages.Insert(message)
	wireId := core.MessageIds{}.Wire(message.Month, message.Id)
	scene.memory.ThreadKeys.Insert(core.ThreadKey{DomainId: domainId, Key: MessageIdKeys{}.Of("<" + wireId + "@" + domainId + ">"), ConversationId: conversationId, MessageId: wireId})
	return conversation, message
}

func Email(inboundId string, from string, rcpts []string) events.EmailReceived {
	return events.EmailReceived{
		InboundId:  inboundId,
		Rcpts:      rcpts,
		RawBlobKey: "example.com/inbound-" + inboundId,
		Headers: events.EmailHeaders{
			MessageId:  inboundId + "@mail.outside.test",
			References: []string{},
			Subject:    "Hello " + inboundId,
			From:       from,
			To:         rcpts,
			Cc:         []string{},
		},
		AuthResults: events.AuthResults{Spf: "pass", Dkim: "pass", Dmarc: "pass"},
	}
}
