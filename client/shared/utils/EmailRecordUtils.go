import (
	"encoding/json"
	"maps"
	"slices"
	"sort"
	"strconv"
	"time"
)

// EmailRecordUtils adds to a message read from the mail API the labels that
// the reader's view needs, as decorateEmail in the reader did.
type EmailRecordUtils struct{}

func (EmailRecordUtils) Email(email Email) Email {
	email.GithubUrl = GitHubNotificationUtils{}.SafeTarget(email.GithubUrl)
	email.Attachments = EmailRecordUtils{}.attachments(email.Attachments)
	email.CalendarInvite = EmailRecordUtils{}.invite(email.CalendarInvite)
	EmailRecordUtils{}.addState(&email)
	EmailRecordUtils{}.addTracking(&email)
	return email
}

// Merged is the message with a patch of its JSON fields applied, as the
// server applies a PUT /inbox/{id}.
func (EmailRecordUtils) Merged(email Email, patch map[string]any) Email {
	fields := map[string]any{}
	encoded, _ := json.Marshal(email)
	_ = json.Unmarshal(encoded, &fields)
	maps.Copy(fields, patch)
	merged := Email{}
	encoded, _ = json.Marshal(fields)
	_ = json.Unmarshal(encoded, &merged)
	return merged
}

// Addresses is a to or cc value, which the server sends as a list or as one
// address, as a list.
func (EmailRecordUtils) Addresses(value any) []string {
	items, listed := value.([]any)
	return TextUtils{}.NonEmpty(append(EmailRecordUtils{}.texts(items), EmailRecordUtils{}.single(value, listed)...))
}

func (EmailRecordUtils) texts(items []any) []string {
	texts := []string{}
	for _, item := range items {
		text, _ := item.(string)
		texts = append(texts, text)
	}
	return texts
}

func (EmailRecordUtils) single(value any, listed bool) []string {
	text, _ := value.(string)
	return map[bool][]string{true: {}, false: {text}}[listed]
}

// InviteAttachments is the invitation's .ics file, or none.
func (EmailRecordUtils) InviteAttachments(email Email) []EmailAttachment {
	return EmailRecordUtils{}.inviteFile(email.CalendarInvite)
}

func (EmailRecordUtils) inviteFile(invite *CalendarInvite) []EmailAttachment {
	if invite == nil || invite.Attachment == nil {
		return []EmailAttachment{}
	}
	return []EmailAttachment{*invite.Attachment}
}

func (EmailRecordUtils) Conversation(email Email) ConversationMessage {
	email = EmailRecordUtils{}.Email(email)
	return ConversationMessage{
		Id:                email.ID,
		Subject:           email.Subject,
		Sender:            email.Sender,
		HeaderFrom:        email.HeaderFrom,
		Recipient:         email.Recipient,
		Folder:            email.Folder,
		SystemFolder:      email.SystemFolder,
		ReceivedAt:        email.ReceivedAt,
		ReceivedLabel:     email.ReceivedLabel,
		Body:              email.Body,
		HtmlBody:          email.HtmlBody,
		InternetMessageId: email.InternetMessageID,
		References:        email.References,
		ReplyTo:           email.ReplyTo,
		Attachments:       email.Attachments,
	}
}

func (EmailRecordUtils) addState(email *Email) {
	email.ReceivedLabel = DateUtils{}.Label(email.ReceivedAt, time.Now())
	email.OpenedLabel = DateUtils{}.Label(email.OpenedAt, time.Now())
	email.HasSpamScore = email.SpamScore != nil
	email.SpamScoreLabel = strconv.FormatFloat(EmailRecordUtils{}.score(email.SpamScore), 'f', 1, 64)
	email.Spam = EmailRecordUtils{}.spam(TextUtils{}.FirstOf(TextUtils{}.FirstOf(email.SystemFolder, email.Folder), "inbox"), email.HasSpamScore, email.SpamReasons != nil)
	email.ReplyToMismatch = slices.Contains(email.SecurityWarnings, "reply_to_mismatch")
}

func (EmailRecordUtils) score(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func (EmailRecordUtils) spam(mailbox string, hasScore bool, hasReasons bool) bool {
	if mailbox == "spam" || hasScore || hasReasons {
		return true
	}
	return false
}

func (EmailRecordUtils) addTracking(email *Email) {
	email.TrackingEvents = EmailRecordUtils{}.trackingEvents(EmailRecordUtils{}.openEvents(email.Opens), EmailRecordUtils{}.clickEvents(email.Clicks))
	openCount := max(len(email.Opens), email.OpenCount)
	clickCount := max(len(email.Clicks), email.ClickCount)
	email.TrackingSummary = EmailRecordUtils{}.trackingSummary(openCount, clickCount)
}

func (EmailRecordUtils) trackingEvents(opens []TrackingEvent, clicks []TrackingEvent) []TrackingEvent {
	events := append(opens, clicks...)
	sort.SliceStable(events, func(left int, right int) bool { return events[left].Timestamp > events[right].Timestamp })
	return events
}

func (EmailRecordUtils) openType() string { return "open" }

func (EmailRecordUtils) openEvents(opens []TrackingHit) []TrackingEvent {
	events := []TrackingEvent{}
	for _, open := range opens {
		events = append(events, TrackingEvent{Type: EmailRecordUtils{}.openType(), EventIcon: "◉", EventLabel: "Opened", Timestamp: open.OpenedAt, TimestampLabel: DateUtils{}.Label(open.OpenedAt, time.Now()), RecipientLabel: EmailRecordUtils{}.recipientLabel(open)})
	}
	return events
}

func (EmailRecordUtils) clickType() string { return "click" }

func (EmailRecordUtils) clickEvents(clicks []TrackingHit) []TrackingEvent {
	events := []TrackingEvent{}
	for _, click := range clicks {
		events = append(events, TrackingEvent{Type: EmailRecordUtils{}.clickType(), EventIcon: "↗", EventLabel: "Clicked", Timestamp: click.ClickedAt, TimestampLabel: DateUtils{}.Label(click.ClickedAt, time.Now()), RecipientLabel: EmailRecordUtils{}.recipientLabel(click), TargetLabel: TextUtils{}.FirstOf(TextUtils{}.FirstOf(click.Title, click.Url), "Link")})
	}
	return events
}

func (EmailRecordUtils) recipientLabel(event TrackingHit) string {
	return TextUtils{}.FirstOf(event.Recipient, "Recipient unavailable")
}

func (EmailRecordUtils) trackingSummary(opens int, clicks int) string {
	if opens == 0 && clicks == 0 {
		return "No activity yet"
	}
	return TextUtils{}.Count(opens, EmailRecordUtils{}.openType(), "opens") + " · " + TextUtils{}.Count(clicks, EmailRecordUtils{}.clickType(), "clicks")
}

func (EmailRecordUtils) attachments(attachments []EmailAttachment) []EmailAttachment {
	labelled := []EmailAttachment{}
	for _, attachment := range attachments {
		labelled = append(labelled, EmailRecordUtils{}.attachment(attachment))
	}
	return labelled
}

func (EmailRecordUtils) attachment(attachment EmailAttachment) EmailAttachment {
	attachment.PreviewKind = AttachmentContentUtils{}.PreviewKind(attachment.ContentType)
	attachment.SizeLabel = SizeUtils{}.Label(int(attachment.Size))
	return attachment
}

func (EmailRecordUtils) invite(invite *CalendarInvite) *CalendarInvite {
	if invite == nil || invite.Attachment == nil {
		return invite
	}
	labelled := *invite
	attachment := EmailRecordUtils{}.attachment(*invite.Attachment)
	labelled.Attachment = &attachment
	return &labelled
}
