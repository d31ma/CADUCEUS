package sending

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/fakes"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

type scene struct {
	world  *fakes.FakeWorld
	sender Sender
	ana    oidc.Caller
}

func newScene(t *testing.T) *scene {
	t.Helper()
	world := fakes.NewFakeWorld()
	world.Directory.Member("ana@example.com", "ana", "Ana")
	world.Directory.Member("bo@example.com", "bo", "Bo")
	world.Directory.Member("cy@example.com", "cy", "Cy")
	world.Directory.OtherDomain("eve@example.org")
	world.Directory.Channel("releases@example.com", "chan1")
	for _, name := range []string{"ana", "bo", "cy"} {
		world.Stores.Members.Save(core.Member{DomainId: "example.com", MemberId: name, LocalPart: name, DisplayName: name, PrimaryAddress: name + "@example.com"})
	}
	for index := 0; index < 10; index++ {
		world.Attachments.Ready("example.com", fmt.Sprintf("att_%d", index), fmt.Sprintf("file-%d.txt", index), 1000)
	}
	ana := oidc.Caller{DomainId: "example.com", MemberId: "ana", DisplayName: "Ana", Email: "Ana@Example.com"}
	return &scene{world: world, sender: NewSender(world.Stores, world.Ports), ana: ana}
}

func (s *scene) send(request core.SendRequest) (core.SendOutcome, error) {
	return s.sender.Send(s.ana, "Bearer ana-token", request)
}

func (s *scene) mustSend(t *testing.T, request core.SendRequest) core.SendOutcome {
	t.Helper()
	outcome, err := s.send(request)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	return outcome
}

func (s *scene) messages() []core.Message {
	return s.world.Stores.Messages.(*core.MemoryMessages).All()
}

func (s *scene) conversations() []core.Conversation {
	return s.world.Stores.Conversations.(*core.MemoryConversations).All()
}

func (s *scene) participants() []core.Participant {
	return s.world.Stores.Participants.(*core.MemoryParticipants).All()
}

func (s *scene) threadKeys() []core.ThreadKey {
	return s.world.Stores.ThreadKeys.(*core.MemoryThreadKeys).All()
}

func (s *scene) mentions() []core.Mention {
	return s.world.Stores.Mentions.(*core.MemoryMentions).All()
}

func (s *scene) stored(t *testing.T, outcome core.SendOutcome) core.Message {
	t.Helper()
	message := outcome.Message.Message
	found, ok, _ := s.world.Stores.Messages.Find("example.com", message.Month, message.Id)
	if !ok {
		t.Fatalf("message %s not stored", message.Id)
	}
	return found
}

func (s *scene) channel(t *testing.T, archived bool) core.Conversation {
	t.Helper()
	conversation := core.Conversation{DomainId: "example.com", Key: "chan1", Kind: "channel", ChannelId: "chan1", Visibility: "public", EmailAddress: "support@example.com", Archived: archived}
	id, _ := s.world.Stores.Conversations.Insert(conversation)
	conversation.Id = id
	for _, member := range []string{"ana", "bo"} {
		s.world.Stores.Participants.Insert(core.Participant{DomainId: "example.com", ConversationId: "cnv_chan1", Kind: "member", MemberId: member, Address: member + "@example.com", Role: "member"})
	}
	return conversation
}

func (s *scene) inboundEmail(t *testing.T) core.Message {
	t.Helper()
	s.world.Stores.Participants.Insert(core.Participant{DomainId: "example.com", ConversationId: "cnv_chan1", Kind: "external", Address: "dan@partner.org", Role: "to"})
	conversation, _, _ := s.world.Stores.Conversations.Find("example.com", "chan1")
	conversation.MessageCount = 1
	s.world.Stores.Conversations.Save(conversation)
	message := core.Message{DomainId: "example.com", ConversationId: "cnv_chan1", Month: "202611", Sequence: 1, AuthorKind: "external", AuthorAddress: "dan@partner.org", Body: "help", BlocksJson: "[]", IsEmail: true,
		Email: core.EmailMeta{MessageIdHeader: "abc@partner.org", References: []string{"r1@partner.org"}, Subject: "Printer broken", From: "dan@partner.org"}, EventsStaged: true}
	id, _ := s.world.Stores.Messages.Insert(message)
	message.Id = id
	return message
}

func wireOf(message core.Message) string {
	return core.MessageIds{}.Wire(message.Month, message.Id)
}

func choice(address string, role string, delivery string, kind string) core.RecipientChoice {
	return core.RecipientChoice{Address: address, Role: role, Delivery: delivery, ExpectedKind: kind}
}

func to(address string, delivery string, kind string) core.RecipientChoice {
	return choice(address, "to", delivery, kind)
}

func request(clientMessageId string, body string, recipients ...core.RecipientChoice) core.SendRequest {
	return core.SendRequest{ClientMessageId: clientMessageId, Body: body, Recipients: recipients}
}

func staged(t *testing.T, fields core.OutboxFields, eventType string) []events.Envelope {
	t.Helper()
	envelopes, err := outbox.Published{}.Staged(fields.Fields(), eventType)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	return envelopes
}

func created(t *testing.T, message core.Message) events.MessageCreated {
	t.Helper()
	payload := events.MessageCreated{}
	_, err := outbox.Published{}.One(message.OutboxFields.Fields(), events.Types{}.MessageCreated(), &payload)
	if err != nil {
		t.Fatalf("message.created: %v", err)
	}
	return payload
}

func emailRequests(t *testing.T, message core.Message) []events.EmailSendRequested {
	t.Helper()
	requests := []events.EmailSendRequested{}
	for _, envelope := range staged(t, message.OutboxFields, events.Types{}.EmailSendRequested()) {
		payload := events.EmailSendRequested{}
		json.Unmarshal(envelope.Payload, &payload)
		requests = append(requests, payload)
	}
	return requests
}

func rowsText(rows []core.DeliveryRecord) string {
	text := ""
	for _, row := range rows {
		text += fmt.Sprintf("%s/%s/%s/%s;", row.Recipient, row.Channel, row.Role, row.Status)
	}
	return text
}

func wantInvalid(t *testing.T, err error, field string) {
	t.Helper()
	invalid := &core.InvalidRequest{}
	if !errors.As(err, &invalid) || invalid.Field != field {
		t.Fatalf("want InvalidRequest on %s, got %#v", field, err)
	}
}

func wantNotFound(t *testing.T, err error) {
	t.Helper()
	if !errors.As(err, new(*core.NotFound)) {
		t.Fatalf("want NotFound, got %#v", err)
	}
}

func wantUnavailable(t *testing.T, err error) {
	t.Helper()
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want Unavailable, got %#v", err)
	}
}
