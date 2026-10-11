import (
	"strings"
)

// SendUtils builds the requests that send or schedule a message and reads
// their outcome, like the send path of the former compose view.
type SendUtils struct{}

// Input is the body of POST /send or PUT /send/scheduled/{id}. Drafts and Sent
// copies never hold tracking URLs: the server adds them at the SMTP boundary.
func (SendUtils) Input(fields MessageFields, choice SendChoice) SendInput {
	input := SendInput{
		From: fields.From, To: RecipientUtils{}.Split(fields.To), Cc: RecipientUtils{}.Split(fields.Cc), Bcc: RecipientUtils{}.Split(fields.Bcc),
		ReplyTo: SendUtils{}.replyTo(RecipientUtils{}.Split(fields.ReplyTo)), InReplyTo: fields.InReplyTo, References: fields.References, Subject: fields.Subject,
		HTML: SendUtils{}.rich(choice.RichText, fields.HTML), Text: fields.Text, Attachments: fields.Attachments,
		OpenTracking: fields.OpenTracking, LinkTracking: fields.LinkTracking, RemindAt: SendUtils{}.optional(fields.RemindAt), IdempotencyKey: choice.IdempotencyKey,
	}
	return SendUtils{}.forMode(input, choice, fields.SourceEmailID)
}

func (SendUtils) scheduled(mode string) bool {
	return strings.HasPrefix(mode, SendUtils{}.names().Schedule)
}

// Route is the page the composer opens after a send.
func (SendUtils) Route(mode string) string {
	return map[bool]string{true: "/scheduled", false: "/inbox"}[SendUtils{}.scheduled(mode)]
}

// Failure is the message for a send the server refused or never saw.
func (SendUtils) Failure(mode string, failure error) string {
	return HTTPUtils{}.Message(failure, map[bool]string{true: "Schedule failed.", false: "Send failed."}[SendUtils{}.scheduled(mode)], "Network error.")
}

// Sent is the undo notice of a send that the server holds for the undo delay.
func (SendUtils) Sent(mode string, answer SendAnswer) string {
	return SendUtils{}.sent(mode == SendUtils{}.names().Archive, answer.Archived)
}

// Sounded says whether a mode plays the send sounds.
func (SendUtils) Sounded(mode string) bool {
	return !SendUtils{}.scheduled(mode)
}

func (SendUtils) forMode(input SendInput, choice SendChoice, source string) SendInput {
	switch {
	case choice.Mode == SendUtils{}.names().Archive:
		input.Archive, input.ArchiveSourceID, input.DelayMs = true, source, choice.DelayMs
	case SendUtils{}.scheduled(choice.Mode):
		input.SendAt = choice.SendAt
	default:
		input.DelayMs = choice.DelayMs
	}
	return input
}

func (SendUtils) replyTo(addresses []string) []string {
	if len(addresses) == 0 {
		return nil
	}
	return []string{strings.Join(addresses, ", ")}
}

func (SendUtils) rich(richText bool, html string) string {
	if richText {
		return html
	}
	return ""
}

func (SendUtils) optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (SendUtils) sent(archiving bool, archived bool) string {
	switch {
	case !archiving:
		return "Message sent."
	case archived:
		return "Sent & archived."
	}
	return "Message sent; source was not archived."
}

func (SendUtils) names() ComposeNames {
	return ComposeNames{Archive: "archive", Schedule: "schedule"}
}
