//go:build blackbox

package blackbox

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

type sendResult struct {
	Message struct {
		MessageId      string `json:"messageId"`
		ConversationId string `json:"conversationId"`
	} `json:"message"`
	Deliveries []struct {
		Recipient string `json:"recipient"`
		Channel   string `json:"channel"`
		Status    string `json:"status"`
	} `json:"deliveries"`
}

type conversationPage struct {
	Items []struct {
		ConversationId string `json:"conversationId"`
		HasExternal    bool   `json:"hasExternal"`
	} `json:"items"`
}

type messagePage struct {
	Items []struct {
		MessageId string `json:"messageId"`
		Body      string `json:"body"`
	} `json:"items"`
}

func uniqueClientId(name string) string {
	return fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
}

func toOne(name string, text string, address string, delivery string, expectedKind string) map[string]any {
	return map[string]any{
		"clientMessageId": uniqueClientId(name),
		"body":            text,
		"recipients":      []map[string]string{{"address": address, "role": "to", "delivery": delivery, "expectedKind": expectedKind}},
	}
}

func send(t *testing.T, stack Stack, token string, body map[string]any, status int) sendResult {
	t.Helper()
	var result sendResult
	reply := stack.Call(t, http.MethodPost, "/api/v1/messaging/messages", token, body).Expect(t, status)
	if status == http.StatusCreated {
		reply.Decode(t, &result)
	}
	return result
}

func deliveryChannels(result sendResult) map[string]string {
	channels := map[string]string{}
	for _, delivery := range result.Deliveries {
		channels[delivery.Recipient] = delivery.Channel
	}
	return channels
}

func conversationHasBody(t *testing.T, stack Stack, token string, conversationId string, text string) bool {
	t.Helper()
	var page messagePage
	path := "/api/v1/messaging/conversations/" + conversationId + "/messages?limit=100"
	stack.Call(t, http.MethodGet, path, token, nil).Expect(t, http.StatusOK).Decode(t, &page)
	for _, message := range page.Items {
		if message.Body == text {
			return true
		}
	}
	return false
}

func externalConversationHasBody(t *testing.T, stack Stack, token string, text string) bool {
	t.Helper()
	var page conversationPage
	stack.Call(t, http.MethodGet, "/api/v1/messaging/conversations?limit=100", token, nil).Expect(t, http.StatusOK).Decode(t, &page)
	for _, conversation := range page.Items {
		if conversation.HasExternal && conversationHasBody(t, stack, token, conversation.ConversationId, text) {
			return true
		}
	}
	return false
}

func TestAcCp4MixedRecipientsGetChatAndEmail(t *testing.T) {
	stack := NewStack(t)
	body := map[string]any{
		"clientMessageId": uniqueClientId("mixed"),
		"body":            "Release notes attached.",
		"recipients": []map[string]string{
			{"address": "bo@example.com", "role": "to", "delivery": "chat", "expectedKind": "member"},
			{"address": "carol@partner.test", "role": "to", "delivery": "email", "expectedKind": "external"},
		},
		"email": map[string]any{"subject": "Release 42"},
	}
	channels := deliveryChannels(send(t, stack, stack.Token(t, "ANA"), body, http.StatusCreated))
	if channels["bo@example.com"] != "chat" || channels["carol@partner.test"] != "email" {
		t.Fatalf("got %+v", channels)
	}
}

func TestAcCp2ChatToAnExternalAddressIsRefused(t *testing.T) {
	stack := NewStack(t)
	body := toOne("refused", "hello", "carol@partner.test", "chat", "external")
	send(t, stack, stack.Token(t, "ANA"), body, http.StatusUnprocessableEntity)
}

func TestAcCp5StaleResolutionIsRefused(t *testing.T) {
	stack := NewStack(t)
	body := toOne("stale", "hello", "bo@example.com", "chat", "external")
	send(t, stack, stack.Token(t, "ANA"), body, http.StatusConflict)
}

func TestAcCp3EmailNeedsASubjectForANewConversation(t *testing.T) {
	stack := NewStack(t)
	body := toOne("nosubject", "hello", "carol@partner.test", "email", "external")
	send(t, stack, stack.Token(t, "ANA"), body, http.StatusBadRequest)
}

func TestAcDm5ChatToAnotherHostedDomainIsRefused(t *testing.T) {
	stack := NewStack(t)
	ana := stack.Token(t, "ANA")
	send(t, stack, ana, toOne("crossdomain-chat", "hello", "eve@example.org", "chat", "otherDomain"), http.StatusUnprocessableEntity)
	send(t, stack, ana, toOne("crossdomain-stale", "hello", "eve@example.org", "chat", "member"), http.StatusConflict)
}

func TestAcDm6EmailToAnotherHostedDomainIsDelivered(t *testing.T) {
	stack := NewStack(t)
	ana := stack.Token(t, "ANA")
	eve := stack.Token(t, "EVE")
	text := uniqueClientId("Hello Eve from example.com")
	body := toOne("crossdomain-email", text, "eve@example.org", "email", "otherDomain")
	body["email"] = map[string]any{"subject": "Across domains"}
	channels := deliveryChannels(send(t, stack, ana, body, http.StatusCreated))
	if channels["eve@example.org"] != "email" {
		t.Fatalf("want an email delivery to eve@example.org, got %+v", channels)
	}
	stack.Eventually(t, 60*time.Second, "Eve to see Ana's email in a conversation with hasExternal true", func() bool {
		return externalConversationHasBody(t, stack, eve, text)
	})
}

func TestAcMs8RetryWithTheSameClientIdCreatesOneMessage(t *testing.T) {
	stack := NewStack(t)
	body := toOne("retry", "once only", "bo@example.com", "chat", "member")
	first := send(t, stack, stack.Token(t, "ANA"), body, http.StatusCreated)
	second := send(t, stack, stack.Token(t, "ANA"), body, http.StatusCreated)
	if first.Message.MessageId != second.Message.MessageId {
		t.Fatalf("a retry created a second message: %s and %s", first.Message.MessageId, second.Message.MessageId)
	}
}

func TestAcMs1OnlyParticipantsCanReadAConversation(t *testing.T) {
	stack := NewStack(t)
	ana := stack.Token(t, "ANA")
	bo := stack.Token(t, "BO")
	eve := stack.Token(t, "EVE")
	result := send(t, stack, ana, toOne("private", "just us", "bo@example.com", "chat", "member"), http.StatusCreated)
	path := "/api/v1/messaging/conversations/" + result.Message.ConversationId + "/messages"
	stack.Call(t, http.MethodGet, path, bo, nil).Expect(t, http.StatusOK)
	stack.Call(t, http.MethodGet, path, eve, nil).Expect(t, http.StatusNotFound)
}
