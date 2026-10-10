import (
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

// EmlImport stores one RFC 822 file in an authorized mailbox without the
// SMTP delivery pipeline: no domain routes, inbox rules, forwarding,
// webhooks, push notifications or reminder resolution run. The exact source
// is kept, and the source and attachments are removed again if the message
// cannot be stored.
type EmlImport struct {
	Emails      *EmailsHelper
	Attachments *AttachmentsHelper
	RawMessages *RawMessagesHelper
}

func (EmlImport) longestSubject() int { return 500 }

// Parsed answers the parsed form of a source as the inbound parser gave it:
// the text (else the HTML as text, else the whole source), the HTML and the
// attachments.
func (EmlImport) Parsed(source []byte) (ParsedMessage, string, string, []ParsedAttachment) {
	message := MimeParser{}.Parse(source)
	raw := string(source)
	if !utf8.ValidString(raw) {
		raw = strings.ToValidUTF8(raw, "�")
	}
	html, found := message.Html()
	if !found && (HtmlText{}).LooksLikeHtml(raw) {
		html = raw
	}
	text := message.Text()
	if text == "" {
		text = HtmlText{}.Stripped(html)
	}
	if text == "" {
		text = raw
	}
	attachments := []ParsedAttachment{}
	for _, attachment := range message.Attachments {
		attachments = append(attachments, ParsedAttachment{
			Filename: TextValue{}.Of(TextValue{}.Or(attachment.Filename, "attachment")), ContentType: TextValue{}.Of(TextValue{}.Or(attachment.ContentType, "application/octet-stream")),
			Content: attachment.Content, Disposition: attachment.Disposition, ContentId: attachment.ContentId,
		})
	}
	return message, text, html, attachments
}

// One stores the source as one message of the mailbox in the folder and
// answers its summary.
func (eml EmlImport) One(source []byte, domain string, mailbox string, folder string) (map[string]any, error) {
	message, text, html, attachments := eml.Parsed(source)
	headers := message.Headers.Flattened()
	id := "import-" + RandomText{}.Hex(16)
	addresses := HeaderAddresses{}
	sender := "unknown@" + domain
	if found := addresses.Found(headers["from"]); len(found) > 0 {
		sender = found[0]
	}
	email := eml.email(headers, id, domain, mailbox, sender, html)
	subject := eml.subject(headers["subject"])
	receivedAt := eml.date(headers["date"])
	for key, value := range map[string]any{
		"subject": subject, "body": text, "folder": folder, "read": false, "starred": false, "receivedAt": receivedAt, "processed": true,
	} {
		email[key] = value
	}
	if html != "" {
		email["htmlBody"] = html
	}
	if len(attachments) > 0 {
		email["hasAttachments"] = true
	}
	if err := eml.stored(id, domain, source, attachments, email); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "subject": subject, "sender": sender, "receivedAt": receivedAt, "attachmentCount": len(attachments)}, nil
}

func (EmlImport) email(headers map[string]any, id string, domain string, mailbox string, sender string, html string) map[string]any {
	addresses := HeaderAddresses{}
	headerFrom, replyTo, warnings := addresses.Sender(sender, headers)
	to := addresses.Found(headers["to"])
	cc := addresses.Found(headers["cc"])
	email := map[string]any{"id": id, "domain": domain, "recipient": mailbox, "sender": sender}
	if len(to) > 0 && to[0] != mailbox {
		email["originalRecipient"] = to[0]
	}
	optional := map[string]bool{"headerFrom": headerFrom != "", "replyTo": len(replyTo) > 0, "securityWarnings": len(warnings) > 0, "to": len(to) > 0, "cc": len(cc) > 0}
	values := map[string]any{"headerFrom": headerFrom, "replyTo": replyTo, "securityWarnings": warnings, "to": to, "cc": cc}
	for field, present := range optional {
		if present {
			email[field] = values[field]
		}
	}
	unsubscribe := UnsubscribeLinks{}.Parsed(headers, html)
	for _, field := range []string{"unsubscribeUrl", "unsubscribeMailto", "unsubscribeBodyUrl"} {
		if unsubscribe[field] != "" {
			email[field] = unsubscribe[field]
		}
	}
	if unsubscribe["unsubscribeOneClick"] == true {
		email["unsubscribeOneClick"] = true
	}
	for key, value := range (Threading{}).ThreadMetadataFromHeaders(headers, id, domain) {
		email[key] = value
	}
	return email
}

func (eml EmlImport) subject(value any) string {
	subject := TextValue{}.Of(TextValue{}.Or(value, "(no subject)"))
	subject = regexp.MustCompile(`[\x00-\x1f\x7f]+`).ReplaceAllString(subject, " ")
	subject = TextValue{}.Prefix(TextValue{}.Trim(TextValue{}.Collapsed(subject)), eml.longestSubject())
	if subject == "" {
		return "(no subject)"
	}
	return subject
}

func (EmlImport) date(value any) string {
	parsed := EpochTime{}.Parse(TextValue{}.Of(TextValue{}.Or(value, "")))
	if math.IsNaN(parsed) {
		return EpochTime{}.NowText()
	}
	return EpochTime{}.Text(int64(parsed))
}

func (eml EmlImport) stored(id string, domain string, source []byte, attachments []ParsedAttachment, email map[string]any) error {
	_, err := eml.RawMessages.SaveForEmail(id, domain, source)
	if err == nil {
		_, err = eml.Attachments.ReplaceForEmail(id, domain, attachments)
	}
	if err == nil {
		_, err = eml.Emails.Put(email)
	}
	if err != nil {
		_ = eml.Attachments.DeleteForEmail(id)
		_ = eml.RawMessages.DeleteForEmail(id)
	}
	return err
}
