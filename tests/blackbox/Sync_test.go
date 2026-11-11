//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
	"time"
)

func settledCursor(t *testing.T, stack Stack, token string, conversationId string) string {
	t.Helper()
	cursor := ""
	stack.Eventually(t, 15*time.Second, "a change id in the conversation list", func() bool {
		cursor = summaryOf(t, stack, token, conversationId).LastChangeId
		return cursor != ""
	})
	return cursor
}

func TestAcOf7NoChangeAfterTheNewestCursor(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	sent := directToBo(t, stack, ana.AccessToken, "sync start "+freshPrefix())
	cursor := settledCursor(t, stack, ana.AccessToken, sent.Message.ConversationId)
	stack.Eventually(t, 15*time.Second, "the conversation to be unchanged after its newest change", func() bool {
		return syncOne(t, stack, ana.AccessToken, sent.Message.ConversationId, cursor).State == "unchanged"
	})
}

func TestAcOf7SyncReturnsTheMessagesChangedAfterTheCursor(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	first := directToBo(t, stack, ana.AccessToken, "sync first "+freshPrefix())
	conversationId := first.Message.ConversationId
	cursor := settledCursor(t, stack, ana.AccessToken, conversationId)
	text := "sync second " + freshPrefix()
	second := replyIn(t, stack, ana.AccessToken, conversationId, text)
	var changed conversationSync
	stack.Eventually(t, 15*time.Second, "the new message in the sync answer", func() bool {
		changed = syncOne(t, stack, fixture(t, stack, "BO").AccessToken, conversationId, cursor)
		_, count := messageNamed(changed, second.Message.MessageId)
		return changed.State == "changed" && count == 1
	})
	message, _ := messageNamed(changed, second.Message.MessageId)
	if message.Body != text || changed.Next == "" || changed.Next == cursor || changed.More {
		t.Fatalf("want the new message and a newer cursor, got %+v", changed)
	}
}

func TestAcOf7EmptyCursorAsksForAReset(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	sent := directToBo(t, stack, ana.AccessToken, "sync reset "+freshPrefix())
	settledCursor(t, stack, ana.AccessToken, sent.Message.ConversationId)
	reset := syncOne(t, stack, ana.AccessToken, sent.Message.ConversationId, "")
	if reset.State != "reset" || reset.Next == "" || len(reset.Messages) != 0 {
		t.Fatalf("want reset with the newest cursor and no messages, got %+v", reset)
	}
}

func TestAcOf7UnknownConversationIsGoneWithoutFailingTheBatch(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	sent := directToBo(t, stack, ana.AccessToken, "sync batch "+freshPrefix())
	known := sent.Message.ConversationId
	answers := syncAll(t, stack, ana.AccessToken, syncCursor(known, ""), syncCursor("cnv_unknown", ""))
	if answers["cnv_unknown"].State != "gone" || answers[known].State != "reset" {
		t.Fatalf("want gone for the unknown id and reset for the known one, got %+v", answers)
	}
}

func TestAcOf11ConversationOfAnotherDomainIsGone(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	sent := directToBo(t, stack, ana.AccessToken, "sync other domain "+freshPrefix())
	eve := fixture(t, stack, "EVE")
	if syncOne(t, stack, eve.AccessToken, sent.Message.ConversationId, "").State != "gone" {
		t.Fatal("an example.com conversation is visible to an example.org token")
	}
}

func TestAcOf7MoreThanFiftyCursorsAreRefused(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	syncReply(t, stack, ana.AccessToken, unknownCursors(51)...).ExpectError(t, http.StatusBadRequest, "invalid_request")
}

func TestAcOf7EditAndReactionComeAsOneCurrentMessage(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	sent := directToBo(t, stack, ana.AccessToken, "sync edit "+freshPrefix())
	conversationId := sent.Message.ConversationId
	messageId := sent.Message.MessageId
	cursor := settledCursor(t, stack, ana.AccessToken, conversationId)
	editReply(t, stack, ana.AccessToken, messageId, "edited once", 1).Expect(t, http.StatusOK)
	stack.Call(t, http.MethodPut, reactionPath(messageId, "thumbs_up"), fixture(t, stack, "BO").AccessToken, nil).Expect(t, http.StatusNoContent)
	var message syncMessage
	stack.Eventually(t, 15*time.Second, "the edit and the reaction in one message", func() bool {
		changed := syncOne(t, stack, ana.AccessToken, conversationId, cursor)
		found, count := messageNamed(changed, messageId)
		message = found
		return count == 1 && found.Version == 2 && found.Reactions["thumbs_up"] == 1
	})
	if message.Body != "edited once" {
		t.Fatalf("want the current body, got %+v", message)
	}
}

func TestAcOf7DeletedMessageComesWithDeletedAtAndNoBody(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	sent := directToBo(t, stack, ana.AccessToken, "sync delete "+freshPrefix())
	conversationId := sent.Message.ConversationId
	cursor := settledCursor(t, stack, ana.AccessToken, conversationId)
	stack.Call(t, http.MethodDelete, "/api/v1/messaging/messages/"+sent.Message.MessageId, ana.AccessToken, nil).Expect(t, http.StatusNoContent)
	stack.Eventually(t, 15*time.Second, "the deletion in the sync answer", func() bool {
		found, count := messageNamed(syncOne(t, stack, ana.AccessToken, conversationId, cursor), sent.Message.MessageId)
		return count == 1 && found.DeletedAt != "" && found.Body == ""
	})
}

func TestAcOf7WrittenAtIsKeptButCreatedAtIsTheServerTime(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	writtenAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	body := toOne("written-offline", "written offline "+freshPrefix(), "bo@example.com", "chat", "member")
	body["writtenAt"] = writtenAt.Format(time.RFC3339)
	sent := send(t, stack, ana.AccessToken, body, http.StatusCreated)
	message := messageById(t, stack, ana.AccessToken, sent.Message.MessageId)
	stored, storedErr := time.Parse(time.RFC3339, message.WrittenAt)
	created, createdErr := time.Parse(time.RFC3339, message.CreatedAt)
	if storedErr != nil || createdErr != nil || !stored.Equal(writtenAt) || !created.After(writtenAt.Add(30*time.Minute)) {
		t.Fatalf("want writtenAt %s and a later createdAt, got %+v", writtenAt, message)
	}
}

func TestAcOf7WrittenAtInTheFutureIsNotKept(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	body := toOne("written-future", "written later "+freshPrefix(), "bo@example.com", "chat", "member")
	body["writtenAt"] = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	sent := send(t, stack, ana.AccessToken, body, http.StatusCreated)
	if message := messageById(t, stack, ana.AccessToken, sent.Message.MessageId); message.WrittenAt != "" {
		t.Fatalf("want no writtenAt for a time after the server's, got %q", message.WrittenAt)
	}
}

func TestAcOf7WrittenAtOlderThanThirtyDaysIsNotKept(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	body := toOne("written-old", "written long ago "+freshPrefix(), "bo@example.com", "chat", "member")
	body["writtenAt"] = time.Now().UTC().Add(-31 * 24 * time.Hour).Format(time.RFC3339)
	sent := send(t, stack, ana.AccessToken, body, http.StatusCreated)
	if message := messageById(t, stack, ana.AccessToken, sent.Message.MessageId); message.WrittenAt != "" {
		t.Fatalf("want no writtenAt older than 30 days, got %q", message.WrittenAt)
	}
}
