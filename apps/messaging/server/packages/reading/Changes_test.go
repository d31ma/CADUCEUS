package reading_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/reading"
)

func TestTheAuthorEditsAMessage(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	view, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text")
	wantNoError(t, err)
	stored := s.stored(message)
	got := fmt.Sprint(view.Message.Body, view.Blocks[0].Spans[0].Text, stored.Body, stored.EditedAt, stored.BlocksJson != "")
	if got != fmt.Sprint("new text", "new text", "new text", "2026-11-10T12:00:00Z", true) {
		t.Fatalf("got %s", got)
	}
}

func TestAnEditStagesMessageEdited(t *testing.T) {
	s := newScene(t)
	conversation, message := s.chat()
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text")
	wantNoError(t, err)
	envelopes := staged(t, s.stored(message).OutboxFields, events.Types{}.MessageEdited())
	payload := events.MessageEdited{}
	decode(t, envelopes[0], &payload)
	want := events.MessageEdited{MessageId: msgId(message), ConversationId: cnvId(conversation), Body: "new text", MemberIds: []string{"ana", "bo"}, EditedAt: "2026-11-10T12:00:00Z"}
	if len(envelopes) != 1 || fmt.Sprint(payload) != fmt.Sprint(want) || envelopes[0].DomainId != "example.com" {
		t.Fatalf("got %d events, %+v", len(envelopes), payload)
	}
}

func TestOnlyTheAuthorEdits(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("bo"), msgId(message), "new text")
	wantError[*core.Forbidden](t, err)
}

func TestADeletedMessageCannotBeEdited(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	message.DeletedAt = "2026-11-10T11:00:00Z"
	s.world.Stores.Messages.Save(message)
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text")
	wantError[*core.Conflict](t, err)
}

func TestAnEditNeedsABody(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "  \n ")
	wantInvalid(t, err, "body")
}

func TestAnEditBodyHasAtMostFortyThousandCharacters(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), strings.Repeat("é", 40001))
	wantInvalid(t, err, "body")
}

func TestAnEditOfFortyThousandCharactersIsAccepted(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), strings.Repeat("é", 40000))
	wantNoError(t, err)
}

func TestAnEditInAnotherDomainIsNotFound(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	caller := s.caller("ana")
	caller.DomainId = "example.org"
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(caller, msgId(message), "new text")
	wantError[*core.NotFound](t, err)
}

func TestAnEditReportsARendererFailure(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.world.Renderer.Failure = errors.New("renderer down")
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text")
	if err == nil || s.stored(message).Body != "hello" {
		t.Fatalf("got %v", err)
	}
}

func TestAnEditReportsABrokenSave(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.breaker.broken["messages.save"] = true
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text")
	wantError[*core.Unavailable](t, err)
}

func TestAnEditReportsABrokenParticipantList(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.breaker.broken["participants.ofConversation"] = true
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text")
	wantError[*core.Unavailable](t, err)
}

func TestAnEditReportsABrokenOutbox(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	message.OutboxFields = corrupt()
	s.world.Stores.Messages.Save(message)
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text")
	if err == nil || s.stored(message).Body != "hello" {
		t.Fatalf("got %v", err)
	}
}

func TestAnEditReportsABrokenView(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.breaker.broken["reactions.ofConversation"] = true
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), "new text")
	wantError[*core.Unavailable](t, err)
}

func TestTheAuthorDeletesAMessage(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	message.AttachmentIds = []string{"att_1"}
	message.AttachmentsJson = `[{"attachmentId":"att_1","fileName":"plan.pdf","mediaType":"application/pdf","size":2048}]`
	s.world.Stores.Messages.Save(message)
	err := reading.NewChanges(s.stores, s.world.Ports).Delete(s.caller("ana"), msgId(message))
	wantNoError(t, err)
	stored := s.stored(message)
	got := fmt.Sprint(stored.Body == "", stored.BlocksJson, len(stored.AttachmentIds), stored.AttachmentsJson, stored.DeletedAt)
	if got != fmt.Sprint(true, "[]", 0, "[]", "2026-11-10T12:00:00Z") {
		t.Fatalf("got %s", got)
	}
}

func TestADeleteStagesMessageDeleted(t *testing.T) {
	s := newScene(t)
	conversation, message := s.chat()
	err := reading.NewChanges(s.stores, s.world.Ports).Delete(s.caller("ana"), msgId(message))
	wantNoError(t, err)
	envelopes := staged(t, s.stored(message).OutboxFields, events.Types{}.MessageDeleted())
	payload := events.MessageDeleted{}
	decode(t, envelopes[0], &payload)
	want := events.MessageDeleted{MessageId: msgId(message), ConversationId: cnvId(conversation), MemberIds: []string{"ana", "bo"}, DeletedAt: "2026-11-10T12:00:00Z"}
	if len(envelopes) != 1 || fmt.Sprint(payload) != fmt.Sprint(want) {
		t.Fatalf("got %d events, %+v", len(envelopes), payload)
	}
}

func TestADeletedMessageStaysAsAPlaceholder(t *testing.T) {
	s := newScene(t)
	conversation, message := s.chat()
	err := reading.NewChanges(s.stores, s.world.Ports).Delete(s.caller("ana"), msgId(message))
	wantNoError(t, err)
	list, err := reading.NewReader(s.stores, s.world.Ports).Messages(s.caller("bo"), cnvId(conversation), "", core.PageRequest{})
	wantNoError(t, err)
	if len(list.Items) != 1 || list.Items[0].Message.DeletedAt == "" || len(list.Items[0].Blocks) != 0 {
		t.Fatalf("got %+v", list.Items)
	}
}

func TestDeletingTwiceStagesOneEvent(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	changes := reading.NewChanges(s.stores, s.world.Ports)
	wantNoError(t, changes.Delete(s.caller("ana"), msgId(message)))
	wantNoError(t, changes.Delete(s.caller("ana"), msgId(message)))
	if count := len(staged(t, s.stored(message).OutboxFields, events.Types{}.MessageDeleted())); count != 1 {
		t.Fatalf("want 1 event, got %d", count)
	}
}

func TestOnlyTheAuthorDeletes(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	err := reading.NewChanges(s.stores, s.world.Ports).Delete(s.caller("bo"), msgId(message))
	wantError[*core.Forbidden](t, err)
}

func TestDeletingAnUnknownMessageIsNotFound(t *testing.T) {
	s := newScene(t)
	s.chat()
	err := reading.NewChanges(s.stores, s.world.Ports).Delete(s.caller("ana"), "msg_202611999999999999")
	wantError[*core.NotFound](t, err)
}

func TestADeleteReportsABrokenParticipantList(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.breaker.broken["participants.ofConversation"] = true
	err := reading.NewChanges(s.stores, s.world.Ports).Delete(s.caller("ana"), msgId(message))
	wantError[*core.Unavailable](t, err)
}

func TestAnEditOverTheStorageBudgetIsRefused(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	_, err := reading.NewChanges(s.stores, s.world.Ports).Edit(s.caller("ana"), msgId(message), strings.Repeat("<", 40000))
	wantError[*core.TooLarge](t, err)
	if s.stored(message).Body != message.Body {
		t.Fatal("the refused edit was stored")
	}
}
