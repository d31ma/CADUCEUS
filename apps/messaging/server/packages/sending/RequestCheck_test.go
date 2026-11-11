package sending

import (
	"fmt"
	"strings"
	"testing"

	"mesail.dev/packages/core"
)

func TestInvalidRequestsNameTheField(t *testing.T) {
	manyAttachments := []string{}
	manyRecipients := []core.RecipientChoice{}
	for index := 0; index <= 10; index++ {
		manyAttachments = append(manyAttachments, fmt.Sprintf("att_%d", index))
	}
	for index := 0; index <= 20; index++ {
		manyRecipients = append(manyRecipients, to(fmt.Sprintf("x%d@partner.org", index), "email", "external"))
	}
	bo := to("bo@example.com", "chat", "member")
	cases := []struct {
		name   string
		change func(*core.SendRequest)
		field  string
	}{
		{"empty client id", func(r *core.SendRequest) { r.ClientMessageId = "" }, "clientMessageId"},
		{"long client id", func(r *core.SendRequest) { r.ClientMessageId = strings.Repeat("x", 201) }, "clientMessageId"},
		{"long body", func(r *core.SendRequest) { r.Body = strings.Repeat("é", 40001) }, "body"},
		{"blank body", func(r *core.SendRequest) { r.Body = " \n " }, "body"},
		{"bad attachment", func(r *core.SendRequest) { r.AttachmentIds = []string{"file_1"} }, "attachmentIds"},
		{"many attachments", func(r *core.SendRequest) { r.AttachmentIds = manyAttachments }, "attachmentIds"},
		{"many recipients", func(r *core.SendRequest) { r.Recipients = manyRecipients }, "recipients"},
		{"bad role", func(r *core.SendRequest) {
			r.Recipients = []core.RecipientChoice{choice("bo@example.com", "from", "chat", "member")}
		}, "recipients"},
		{"bad delivery", func(r *core.SendRequest) {
			r.Recipients = []core.RecipientChoice{to("bo@example.com", "fax", "member")}
		}, "recipients"},
		{"bad kind", func(r *core.SendRequest) {
			r.Recipients = []core.RecipientChoice{to("bo@example.com", "chat", "robot")}
		}, "recipients"},
		{"blank address", func(r *core.SendRequest) { r.Recipients = []core.RecipientChoice{to("  ", "chat", "member")} }, "recipients"},
		{"duplicate address", func(r *core.SendRequest) {
			r.Recipients = []core.RecipientChoice{bo, to("BO@example.com", "chat", "member")}
		}, "recipients"},
		{"chat bcc", func(r *core.SendRequest) {
			r.Recipients = []core.RecipientChoice{choice("bo@example.com", "bcc", "chat", "member")}
		}, "recipients"},
		{"nobody", func(r *core.SendRequest) { r.Recipients = nil }, "recipients"},
		{"also to conversation without a thread", func(r *core.SendRequest) { r.AlsoToConversation = true }, "alsoToConversation"},
	}
	for _, testCase := range cases {
		s := newScene(t)
		sendRequest := request("c1", "hi", bo)
		testCase.change(&sendRequest)
		_, err := s.send(sendRequest)
		if err == nil {
			t.Fatalf("%s: want an error", testCase.name)
		}
		wantInvalid(t, err, testCase.field)
		if len(s.world.Directory.Authorization) != 0 {
			t.Fatalf("%s: an invalid request must not reach the directory", testCase.name)
		}
	}
}

func TestLimitsAreInclusive(t *testing.T) {
	s := newScene(t)
	sendRequest := request(strings.Repeat("x", 200), strings.Repeat("é", 40000), to("bo@example.com", "chat", "member"))
	sendRequest.AttachmentIds = []string{}
	for index := 0; index < 10; index++ {
		sendRequest.AttachmentIds = append(sendRequest.AttachmentIds, fmt.Sprintf("att_%d", index))
	}
	s.mustSend(t, sendRequest)
}
