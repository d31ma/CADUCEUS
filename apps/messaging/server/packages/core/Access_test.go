package core

import (
	"errors"
	"testing"

	"mesail.dev/packages/oidc"
)

func accessWorld(t *testing.T) (Stores, Conversation, oidc.Caller) {
	t.Helper()
	stores := MemoryStores{}.New()
	conversation := Conversation{DomainId: "example.com", Kind: "direct"}
	id, _ := stores.Conversations.Insert(conversation)
	conversation.Id = id
	stores.Participants.Insert(Participant{DomainId: "example.com", ConversationId: ConversationIds{}.Wire(conversation), MemberId: "ana", Kind: "member"})
	return stores, conversation, oidc.Caller{DomainId: "example.com", MemberId: "ana"}
}

func TestAParticipantReadsTheConversation(t *testing.T) {
	stores, conversation, ana := accessWorld(t)
	found, err := NewAccess(stores).Conversation(ana, ConversationIds{}.Wire(conversation))
	if err != nil || found.Id != conversation.Id {
		t.Fatalf("got %+v %v", found, err)
	}
}

func TestAnotherMemberGetsNotFound(t *testing.T) {
	stores, conversation, _ := accessWorld(t)
	_, err := NewAccess(stores).Conversation(oidc.Caller{DomainId: "example.com", MemberId: "bo"}, ConversationIds{}.Wire(conversation))
	if !errors.As(err, new(*NotFound)) {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestAnotherDomainGetsNotFound(t *testing.T) {
	stores, conversation, _ := accessWorld(t)
	_, err := NewAccess(stores).Conversation(oidc.Caller{DomainId: "example.org", MemberId: "ana"}, ConversationIds{}.Wire(conversation))
	if !errors.As(err, new(*NotFound)) {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestAPublicChannelIsReadableButNotPostableWithoutJoining(t *testing.T) {
	stores := MemoryStores{}.New()
	stores.Conversations.Insert(Conversation{DomainId: "example.com", Key: "chan1", Kind: "channel", Visibility: "public"})
	bo := oidc.Caller{DomainId: "example.com", MemberId: "bo"}
	_, err := NewAccess(stores).Conversation(bo, "cnv_chan1")
	if err != nil {
		t.Fatalf("public channel not readable: %v", err)
	}
	_, _, err = NewAccess(stores).Postable(bo, "cnv_chan1")
	if !errors.As(err, new(*NotFound)) {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestAnArchivedChannelIsReadOnly(t *testing.T) {
	stores := MemoryStores{}.New()
	stores.Conversations.Insert(Conversation{DomainId: "example.com", Key: "chan1", Kind: "channel", Visibility: "private", Archived: true})
	stores.Participants.Insert(Participant{DomainId: "example.com", ConversationId: "cnv_chan1", MemberId: "bo"})
	_, _, err := NewAccess(stores).Postable(oidc.Caller{DomainId: "example.com", MemberId: "bo"}, "cnv_chan1")
	if !errors.As(err, new(*Conflict)) {
		t.Fatalf("want Conflict, got %v", err)
	}
}

func TestMessageIdsRoundTrip(t *testing.T) {
	month, stored, err := MessageIds{}.Parse(MessageIds{}.Wire("202611", "abc"))
	if err != nil || month != "202611" || stored != "abc" {
		t.Fatalf("got %s %s %v", month, stored, err)
	}
	if _, _, err := (MessageIds{}).Parse("msg_2026"); err == nil {
		t.Fatal("a short id was accepted")
	}
}

type failingMessages struct {
	MessageStore
}

func (failingMessages) Find(domainId string, month string, id string) (Message, bool, error) {
	return Message{}, false, errors.New("down")
}

func TestAParticipantReadsAMessageOfTheConversation(t *testing.T) {
	stores, conversation, ana := accessWorld(t)
	id, _ := stores.Messages.Insert(Message{DomainId: "example.com", ConversationId: ConversationIds{}.Wire(conversation), Month: "202611"})
	message, found, err := NewAccess(stores).Message(ana, MessageIds{}.Wire("202611", id))
	if err != nil || message.Id != id || found.Id != conversation.Id {
		t.Fatalf("got %+v %+v %v", message, found, err)
	}
	_, _, err = NewAccess(stores).Message(oidc.Caller{DomainId: "example.com", MemberId: "bo"}, MessageIds{}.Wire("202611", id))
	if !errors.As(err, new(*NotFound)) {
		t.Fatalf("another member: want NotFound, got %v", err)
	}
}

func TestAMissingOrMalformedMessageIsNotFound(t *testing.T) {
	stores, _, ana := accessWorld(t)
	for _, messageId := range []string{"msg_x", MessageIds{}.Wire("202611", "000000000009")} {
		_, _, err := NewAccess(stores).Message(ana, messageId)
		if !errors.As(err, new(*NotFound)) {
			t.Fatalf("%s: want NotFound, got %v", messageId, err)
		}
	}
}

func TestABrokenMessageStoreIsUnavailable(t *testing.T) {
	stores, _, ana := accessWorld(t)
	stores.Messages = failingMessages{stores.Messages}
	_, _, err := NewAccess(stores).Message(ana, MessageIds{}.Wire("202611", "000000000001"))
	if !errors.As(err, new(*Unavailable)) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestAttachmentUsesAreFoundPerDomain(t *testing.T) {
	uses := NewMemoryAttachmentUses()
	uses.Insert(AttachmentUse{DomainId: "example.com", AttachmentId: "att_1", MessageId: "msg_1"})
	uses.Insert(AttachmentUse{DomainId: "example.org", AttachmentId: "att_1", MessageId: "msg_2"})
	uses.Insert(AttachmentUse{DomainId: "example.com", AttachmentId: "att_2", MessageId: "msg_3"})
	found, err := uses.ByAttachment("example.com", "att_1")
	if err != nil || len(found) != 1 || found[0].MessageId != "msg_1" || len(uses.All()) != 3 {
		t.Fatalf("got %+v %v", found, err)
	}
}
