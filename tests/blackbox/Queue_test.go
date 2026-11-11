//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
)

func TestAcOf4RepeatedSendIsAcceptedOnce(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	body := toOne("queued-twice", "queued twice "+freshPrefix(), "bo@example.com", "chat", "member")
	first := send(t, stack, ana.AccessToken, body, http.StatusCreated)
	second := send(t, stack, ana.AccessToken, body, http.StatusCreated)
	if first.Message.MessageId == "" || first.Message.MessageId != second.Message.MessageId {
		t.Fatalf("want one message for one clientMessageId, got %s and %s", first.Message.MessageId, second.Message.MessageId)
	}
}

func TestAcOf8EditWithAnOldVersionIsAConflict(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	messageId := directToBo(t, stack, ana.AccessToken, "conflict "+freshPrefix()).Message.MessageId
	var edited syncMessage
	editReply(t, stack, ana.AccessToken, messageId, "from the laptop", 1).Expect(t, http.StatusOK).Decode(t, &edited)
	var conflict editConflict
	editReply(t, stack, ana.AccessToken, messageId, "from the offline phone", 1).Expect(t, http.StatusConflict).Decode(t, &conflict)
	if edited.Version != 2 || conflict.Code != "edit_conflict" || conflict.Current.Body != "from the laptop" || conflict.Current.Version != 2 {
		t.Fatalf("want version 2 and a conflict naming the laptop text, got %+v and %+v", edited, conflict)
	}
}

func TestAcOf8EditWithoutAVersionStillWorks(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	messageId := directToBo(t, stack, ana.AccessToken, "no version "+freshPrefix()).Message.MessageId
	request := map[string]string{"body": "edited online"}
	stack.Call(t, http.MethodPatch, "/api/v1/messaging/messages/"+messageId, ana.AccessToken, request).Expect(t, http.StatusOK)
}

func TestAcOf8DeletionWinsOverEditsAndReactions(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	bo := fixture(t, stack, "BO")
	messageId := directToBo(t, stack, ana.AccessToken, "deleted "+freshPrefix()).Message.MessageId
	path := "/api/v1/messaging/messages/" + messageId
	stack.Call(t, http.MethodDelete, path, ana.AccessToken, nil).Expect(t, http.StatusNoContent)
	stack.Call(t, http.MethodDelete, path, ana.AccessToken, nil).Expect(t, http.StatusNoContent)
	editReply(t, stack, ana.AccessToken, messageId, "too late", 2).ExpectError(t, http.StatusConflict, "message_deleted")
	stack.Call(t, http.MethodPut, reactionPath(messageId, "heart"), bo.AccessToken, nil).ExpectError(t, http.StatusConflict, "message_deleted")
}

func TestAcOf8ReactionsAreASetPerPerson(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	bo := fixture(t, stack, "BO")
	messageId := directToBo(t, stack, ana.AccessToken, "reactions "+freshPrefix()).Message.MessageId
	path := reactionPath(messageId, "eyes")
	stack.Call(t, http.MethodPut, path, bo.AccessToken, nil).Expect(t, http.StatusNoContent)
	stack.Call(t, http.MethodPut, path, bo.AccessToken, nil).Expect(t, http.StatusNoContent)
	if count := messageById(t, stack, ana.AccessToken, messageId).Reactions["eyes"]; count != 1 {
		t.Fatalf("want one eyes reaction after two adds, got %d", count)
	}
	stack.Call(t, http.MethodDelete, path, bo.AccessToken, nil).Expect(t, http.StatusNoContent)
	stack.Call(t, http.MethodDelete, path, bo.AccessToken, nil).Expect(t, http.StatusNoContent)
	if count := messageById(t, stack, ana.AccessToken, messageId).Reactions["eyes"]; count != 0 {
		t.Fatalf("want no eyes reaction after two removes, got %d", count)
	}
}

func TestAcOf8ReadPositionOnlyMovesForward(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	bo := fixture(t, stack, "BO")
	first := directToBo(t, stack, ana.AccessToken, "read first "+freshPrefix())
	conversationId := first.Message.ConversationId
	second := replyIn(t, stack, ana.AccessToken, conversationId, "read second "+freshPrefix())
	path := "/api/v1/messaging/conversations/" + conversationId + "/read"
	stack.Call(t, http.MethodPut, path, bo.AccessToken, map[string]string{"messageId": second.Message.MessageId}).Expect(t, http.StatusNoContent)
	stack.Call(t, http.MethodPut, path, bo.AccessToken, map[string]string{"messageId": first.Message.MessageId}).Expect(t, http.StatusNoContent)
	if read := summaryOf(t, stack, bo.AccessToken, conversationId).ReadMessageId; read != second.Message.MessageId {
		t.Fatalf("want the read position to stay at %s, got %s", second.Message.MessageId, read)
	}
}
