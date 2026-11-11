package reading_test

import (
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/reading"
)

func (s *scene) attach(message core.Message, attachmentId string) {
	s.world.Stores.AttachmentUses.Insert(core.AttachmentUse{DomainId: message.DomainId, AttachmentId: attachmentId, ConversationId: message.ConversationId, MessageId: msgId(message)})
}

func (s *scene) attachedChat() core.Message {
	conversation := s.conversation(core.Conversation{MessageCount: 1, CreatedMonth: "202611"})
	s.join(conversation, core.Participant{MemberId: "ana"})
	s.join(conversation, core.Participant{MemberId: "bo"})
	message := s.post(conversation, core.Message{Sequence: 1, AttachmentIds: []string{"att_1"}})
	s.attach(message, "att_1")
	return message
}

func TestAParticipantCanReadAnAttachment(t *testing.T) {
	s := newScene(t)
	s.attachedChat()
	wantNoError(t, reading.NewAttachments(s.stores).Access(s.caller("bo"), "att_1"))
}

func TestAnAttachmentOfAPublicChannelIsReadableByTheDomain(t *testing.T) {
	s := newScene(t)
	channel := s.conversation(core.Conversation{Kind: "channel", Key: "chan1", Visibility: "public", MessageCount: 1})
	message := s.post(channel, core.Message{Sequence: 1, AttachmentIds: []string{"att_1"}})
	s.attach(message, "att_1")
	wantNoError(t, reading.NewAttachments(s.stores).Access(s.caller("cy"), "att_1"))
}

func TestAnAttachmentIsHiddenFromOthers(t *testing.T) {
	s := newScene(t)
	s.attachedChat()
	attachments := reading.NewAttachments(s.stores)
	wantError[*core.NotFound](t, attachments.Access(s.caller("cy"), "att_1"))
	wantError[*core.NotFound](t, attachments.Access(s.caller("bo"), "att_2"))
	stranger := s.caller("bo")
	stranger.DomainId = "example.org"
	wantError[*core.NotFound](t, attachments.Access(stranger, "att_1"))
}

func TestOneReadableUseIsEnough(t *testing.T) {
	s := newScene(t)
	s.attachedChat()
	conversation := s.conversation(core.Conversation{MessageCount: 1, CreatedMonth: "202611"})
	s.join(conversation, core.Participant{MemberId: "cy"})
	forwarded := s.post(conversation, core.Message{Sequence: 1, AttachmentIds: []string{"att_1"}})
	s.attach(forwarded, "att_1")
	wantNoError(t, reading.NewAttachments(s.stores).Access(s.caller("cy"), "att_1"))
}

func TestAnAttachmentOfADeletedMessageIsHidden(t *testing.T) {
	s := newScene(t)
	message := s.attachedChat()
	message.AttachmentIds = []string{}
	message.DeletedAt = "2026-11-10T12:00:00Z"
	s.world.Stores.Messages.Save(message)
	wantError[*core.NotFound](t, reading.NewAttachments(s.stores).Access(s.caller("bo"), "att_1"))
}

func TestAnAttachmentReportsABrokenUseStore(t *testing.T) {
	s := newScene(t)
	s.attachedChat()
	s.breaker.broken["attachmentUses.byAttachment"] = true
	wantError[*core.Unavailable](t, reading.NewAttachments(s.stores).Access(s.caller("bo"), "att_1"))
}

func TestAnAttachmentReportsABrokenMessageStore(t *testing.T) {
	s := newScene(t)
	s.attachedChat()
	s.breaker.broken["messages.find"] = true
	wantError[*core.Unavailable](t, reading.NewAttachments(s.stores).Access(s.caller("bo"), "att_1"))
}
