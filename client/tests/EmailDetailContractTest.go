import (
	"encoding/json"
)

// EmailDetailContractTest checks the reader boundary independently of HTTP
// wiring: full content survives detail reads while list models stay summaries.
// @Test(ShellTestSetup, {"partition":"tests"})
type EmailDetailContractTest struct {
	behaviors *BehaviorContext
}

func (test *EmailDetailContractTest) detailAnswer() string {
	return `{"id":"m1","sender":"sender@example.net","body":"Plain message","htmlBody":"<p>Message</p>","rfcMessageId":"<m1@example.net>","references":["<root@example.net>"],"replyTo":["reply@example.net"],"attachments":[{"id":"a1","filename":"notes.txt"}],"securityWarnings":["reply_to_mismatch"]}`
}

func (test *EmailDetailContractTest) TestDetailDecodesReplyAndContentFields() {
	answer := test.detailAnswer()
	detail := EmailDetail{}
	Assert.Equals(nil, json.Unmarshal([]byte(answer), &detail))
	Assert.Equals("Plain message", detail.Body)
	Assert.Equals("<p>Message</p>", detail.HtmlBody)
	Assert.Equals("<m1@example.net>", detail.InternetMessageID)
	Assert.Equals([]string{"<root@example.net>"}, detail.References)
	Assert.Equals([]string{"reply@example.net"}, detail.ReplyTo)
	Assert.Equals("notes.txt", detail.Attachments[0].Filename)
	Assert.IsTrue(EmailRecordUtils{}.Email(detail).ReplyToMismatch)

	summary := Email{}
	Assert.Equals(nil, json.Unmarshal([]byte(answer), &summary))
	encoded, failure := json.Marshal(summary)
	Assert.Equals(nil, failure)
	fields := map[string]any{}
	Assert.Equals(nil, json.Unmarshal(encoded, &fields))
	Assert.Equals("m1", fields["id"])
}

// @ValueSource(["body", "htmlBody", "rfcMessageId", "references", "replyTo", "attachments", "securityWarnings"])
func (test *EmailDetailContractTest) TestSummaryExcludesDetailFields(key string) {
	summary := Email{}
	Assert.Equals(nil, json.Unmarshal([]byte(test.detailAnswer()), &summary))
	encoded, failure := json.Marshal(summary)
	Assert.Equals(nil, failure)
	fields := map[string]any{}
	Assert.Equals(nil, json.Unmarshal(encoded, &fields))
	_, present := fields[key]
	Assert.Equals(false, present)
}

func (test *EmailDetailContractTest) TestFlagPatchPreservesReplyAndContentFields() {
	detail := EmailDetail{ID: "m1", Body: "Plain message", HtmlBody: "<p>Message</p>", InternetMessageID: "<m1@example.net>", References: []string{"<root@example.net>"}, ReplyTo: []string{"reply@example.net"}, Attachments: []EmailAttachment{{Id: "a1", Filename: "notes.txt"}}}
	changed := EmailRecordUtils{}.Merged(detail, map[string]any{"read": true, "folder": "archive"})
	Assert.IsTrue(changed.Read)
	Assert.Equals("archive", changed.Folder)
	Assert.Equals(detail.Body, changed.Body)
	Assert.Equals(detail.HtmlBody, changed.HtmlBody)
	Assert.Equals(detail.InternetMessageID, changed.InternetMessageID)
	Assert.Equals(detail.References, changed.References)
	Assert.Equals(detail.ReplyTo, changed.ReplyTo)
	Assert.Equals("notes.txt", changed.Attachments[0].Filename)
}

func (test *EmailDetailContractTest) TestThreadDetailKeepsConversationReplyData() {
	answer := `[{"id":"t1","subject":"Plans","emails":[{"id":"m1","body":"Earlier text","htmlBody":"<p>Earlier text</p>","rfcMessageId":"<m1@example.net>","references":["<root@example.net>"],"replyTo":["reply@example.net"]}]}]`
	threads := []EmailDetailThread{}
	Assert.Equals(nil, json.Unmarshal([]byte(answer), &threads))
	message := EmailRecordUtils{}.Conversation(threads[0].Emails[0])
	Assert.Equals("Earlier text", message.Body)
	Assert.Equals("<p>Earlier text</p>", message.HtmlBody)
	Assert.Equals("<m1@example.net>", message.InternetMessageId)
	Assert.Equals([]string{"<root@example.net>"}, message.References)
	Assert.Equals([]string{"reply@example.net"}, message.ReplyTo)
}
