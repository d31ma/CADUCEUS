//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
)

type syncMessage struct {
	MessageId   string         `json:"messageId"`
	Body        string         `json:"body"`
	Version     int            `json:"version"`
	WrittenAt   string         `json:"writtenAt"`
	CreatedAt   string         `json:"createdAt"`
	DeletedAt   string         `json:"deletedAt"`
	Reactions   map[string]int `json:"reactions"`
	MyReactions []string       `json:"myReactions"`
}

type conversationSync struct {
	ConversationId string        `json:"conversationId"`
	State          string        `json:"state"`
	Messages       []syncMessage `json:"messages"`
	Next           string        `json:"next"`
	More           bool          `json:"more"`
}

type conversationSummary struct {
	ConversationId string `json:"conversationId"`
	LastChangeId   string `json:"lastChangeId"`
	ReadMessageId  string `json:"readMessageId"`
}

type editConflict struct {
	Code    string      `json:"code"`
	Current syncMessage `json:"current"`
}

func syncCursor(conversationId string, after string) map[string]string {
	return map[string]string{"conversationId": conversationId, "after": after}
}

func unknownCursors(count int) []map[string]string {
	cursors := []map[string]string{}
	for index := 0; index < count; index++ {
		cursors = append(cursors, syncCursor("cnv_unknown", ""))
	}
	return cursors
}

func syncReply(t *testing.T, stack Stack, token string, cursors ...map[string]string) Reply {
	t.Helper()
	request := map[string]any{"cursors": cursors}
	return stack.Call(t, http.MethodPost, "/api/v1/messaging/sync", token, request)
}

func syncAll(t *testing.T, stack Stack, token string, cursors ...map[string]string) map[string]conversationSync {
	t.Helper()
	var response struct {
		Conversations []conversationSync `json:"conversations"`
	}
	syncReply(t, stack, token, cursors...).Expect(t, http.StatusOK).Decode(t, &response)
	byId := map[string]conversationSync{}
	for _, conversation := range response.Conversations {
		byId[conversation.ConversationId] = conversation
	}
	return byId
}

func syncOne(t *testing.T, stack Stack, token string, conversationId string, after string) conversationSync {
	t.Helper()
	return syncAll(t, stack, token, syncCursor(conversationId, after))[conversationId]
}

func summaryOf(t *testing.T, stack Stack, token string, conversationId string) conversationSummary {
	t.Helper()
	var page struct {
		Items []conversationSummary `json:"items"`
	}
	stack.Call(t, http.MethodGet, "/api/v1/messaging/conversations?limit=100", token, nil).Expect(t, http.StatusOK).Decode(t, &page)
	for _, item := range page.Items {
		if item.ConversationId == conversationId {
			return item
		}
	}
	t.Fatalf("conversation %s is not in the first page of the list", conversationId)
	return conversationSummary{}
}

func messageNamed(conversation conversationSync, messageId string) (syncMessage, int) {
	found := syncMessage{}
	count := 0
	for _, message := range conversation.Messages {
		if message.MessageId == messageId {
			found = message
			count++
		}
	}
	return found, count
}

func directToBo(t *testing.T, stack Stack, anaToken string, text string) sendResult {
	t.Helper()
	return send(t, stack, anaToken, toOne("offline", text, "bo@example.com", "chat", "member"), http.StatusCreated)
}

func replyIn(t *testing.T, stack Stack, token string, conversationId string, text string) sendResult {
	t.Helper()
	body := map[string]any{"clientMessageId": uniqueClientId("offline-reply"), "conversationId": conversationId, "body": text}
	return send(t, stack, token, body, http.StatusCreated)
}

func messageById(t *testing.T, stack Stack, token string, messageId string) syncMessage {
	t.Helper()
	var message syncMessage
	stack.Call(t, http.MethodGet, "/api/v1/messaging/messages/"+messageId, token, nil).Expect(t, http.StatusOK).Decode(t, &message)
	return message
}

func editReply(t *testing.T, stack Stack, token string, messageId string, body string, ifVersion int) Reply {
	t.Helper()
	request := map[string]any{"body": body, "ifVersion": ifVersion}
	return stack.Call(t, http.MethodPatch, "/api/v1/messaging/messages/"+messageId, token, request)
}

func reactionPath(messageId string, reaction string) string {
	return "/api/v1/messaging/messages/" + messageId + "/reactions/" + reaction
}

func keptSignIn(t *testing.T, stack Stack, login string, password string, kept bool) session {
	t.Helper()
	var signedIn session
	request := map[string]any{"login": login, "password": password, "keepSignedIn": kept}
	stack.Call(t, http.MethodPost, "/api/v1/identity/session", "", request).Expect(t, http.StatusOK).Decode(t, &signedIn)
	return signedIn
}
