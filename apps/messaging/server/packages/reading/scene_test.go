package reading_test

import (
	"encoding/json"
	"errors"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/fakes"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

type scene struct {
	t       *testing.T
	world   *fakes.FakeWorld
	breaker *breaker
	stores  core.Stores
}

func newScene(t *testing.T) *scene {
	t.Helper()
	world := fakes.NewFakeWorld()
	broken := &breaker{broken: map[string]bool{}}
	return &scene{t: t, world: world, breaker: broken, stores: broken.wrap(world.Stores)}
}

func (s *scene) caller(memberId string) oidc.Caller {
	return oidc.Caller{DomainId: "example.com", MemberId: memberId}
}

func (s *scene) conversation(conversation core.Conversation) core.Conversation {
	if conversation.DomainId == "" {
		conversation.DomainId = "example.com"
	}
	if conversation.Kind == "" {
		conversation.Kind = "direct"
	}
	id, _ := s.world.Stores.Conversations.Insert(conversation)
	conversation.Id = id
	return conversation
}

func (s *scene) join(conversation core.Conversation, participant core.Participant) core.Participant {
	participant.DomainId = conversation.DomainId
	participant.ConversationId = cnvId(conversation)
	if participant.Kind == "" {
		participant.Kind = "member"
	}
	id, _ := s.world.Stores.Participants.Insert(participant)
	participant.Id = id
	return participant
}

func (s *scene) post(conversation core.Conversation, message core.Message) core.Message {
	message.DomainId = conversation.DomainId
	message.ConversationId = cnvId(conversation)
	if message.Month == "" {
		message.Month = "202611"
	}
	if message.AuthorKind == "" {
		message.AuthorKind = "member"
	}
	id, _ := s.world.Stores.Messages.Insert(message)
	message.Id = id
	return message
}

func (s *scene) messages() []core.Message {
	return s.world.Stores.Messages.(*core.MemoryMessages).All()
}

func (s *scene) stored(message core.Message) core.Message {
	found, _, _ := s.world.Stores.Messages.Find(message.DomainId, message.Month, message.Id)
	return found
}

func (s *scene) participants() []core.Participant {
	return s.world.Stores.Participants.(*core.MemoryParticipants).All()
}

func (s *scene) reactions() []core.Reaction {
	return s.world.Stores.Reactions.(*core.MemoryReactions).All()
}

func (s *scene) chat() (core.Conversation, core.Message) {
	conversation := s.conversation(core.Conversation{MessageCount: 1, CreatedMonth: "202611"})
	s.join(conversation, core.Participant{MemberId: "ana", DisplayName: "Ana"})
	s.join(conversation, core.Participant{MemberId: "bo", DisplayName: "Bo"})
	s.join(conversation, core.Participant{Kind: "external", Address: "x@ext.org"})
	message := s.post(conversation, core.Message{Sequence: 1, AuthorMemberId: "ana", Body: "hello", BlocksJson: `[{"kind":"paragraph","spans":[{"text":"hello"}]}]`})
	return conversation, message
}

func cnvId(conversation core.Conversation) string {
	return core.ConversationIds{}.Wire(conversation)
}

func msgId(message core.Message) string {
	return core.MessageIds{}.Wire(message.Month, message.Id)
}

func staged(t *testing.T, fields core.OutboxFields, eventType string) []events.Envelope {
	t.Helper()
	envelopes, err := outbox.Published{}.Staged(fields.Fields(), eventType)
	if err != nil {
		t.Fatalf("read staged events: %v", err)
	}
	return envelopes
}

func decode(t *testing.T, envelope events.Envelope, payload any) {
	t.Helper()
	if err := json.Unmarshal(envelope.Payload, payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
}

func wantError[T error](t *testing.T, err error) T {
	t.Helper()
	var target T
	if !errors.As(err, &target) {
		t.Fatalf("want %T, got %v", target, err)
	}
	return target
}

func wantInvalid(t *testing.T, err error, field string) {
	t.Helper()
	invalid := wantError[*core.InvalidRequest](t, err)
	if invalid.Field != field {
		t.Fatalf("want field %q, got %q", field, invalid.Field)
	}
}

func wantNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func corrupt() core.OutboxFields {
	return core.OutboxFields{PendingEvents: []string{"not json"}, OutboxPending: 1}
}
