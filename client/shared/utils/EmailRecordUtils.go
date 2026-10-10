import (
	"slices"
	"sort"
	"strconv"
)

// EmailRecordUtils turns a mail API record into the reader's view model and
// adds the labels that the view needs, as decorateEmail in the reader did.
type EmailRecordUtils struct{}

func (EmailRecordUtils) Email(record map[string]any) Email {
	email := EmailRecordUtils{}.addresses(record)
	EmailRecordUtils{}.addContent(&email, record)
	EmailRecordUtils{}.addState(&email, record)
	EmailRecordUtils{}.addTracking(&email, record)
	return email
}

func (EmailRecordUtils) Invite(record map[string]any) *CalendarInvite {
	return EmailRecordUtils{}.invite(RecordUtils{}.Record(record, "calendarInvite"))
}

func (EmailRecordUtils) Conversation(record map[string]any) ConversationMessage {
	email := EmailRecordUtils{}.Email(record)
	return ConversationMessage{
		Id:                email.Id,
		Subject:           email.Subject,
		Sender:            email.Sender,
		HeaderFrom:        email.HeaderFrom,
		Recipient:         email.Recipient,
		Folder:            email.Folder,
		SystemFolder:      email.SystemFolder,
		ReceivedAt:        email.ReceivedAt,
		Body:              email.Body,
		HtmlBody:          email.HtmlBody,
		InternetMessageId: email.InternetMessageId,
		References:        email.References,
		ReplyTo:           email.ReplyTo,
		Attachments:       email.Attachments,
	}
}

func (EmailRecordUtils) idKey() string { return "id" }

func (EmailRecordUtils) recipientKey() string { return "recipient" }

func (EmailRecordUtils) addresses(record map[string]any) Email {
	return Email{
		Id:                RecordUtils{}.Text(record, EmailRecordUtils{}.idKey()),
		Subject:           RecordUtils{}.Text(record, "subject"),
		Sender:            RecordUtils{}.Text(record, "sender"),
		HeaderFrom:        RecordUtils{}.Text(record, "headerFrom"),
		Recipient:         RecordUtils{}.Text(record, EmailRecordUtils{}.recipientKey()),
		OriginalRecipient: RecordUtils{}.Text(record, "originalRecipient"),
		To:                RecordUtils{}.Texts(record, "to"),
		Cc:                RecordUtils{}.Texts(record, "cc"),
		ReplyTo:           RecordUtils{}.Texts(record, "replyTo"),
		InternetMessageId: RecordUtils{}.Text(record, "rfcMessageId"),
		MessageId:         RecordUtils{}.Text(record, "messageId"),
		References:        RecordUtils{}.Texts(record, "references"),
		ThreadId:          RecordUtils{}.Text(record, "threadId"),
	}
}

func (EmailRecordUtils) addContent(email *Email, record map[string]any) {
	email.Body = RecordUtils{}.Text(record, "body")
	email.HtmlBody = RecordUtils{}.Text(record, "htmlBody")
	email.HasRawSource = RecordUtils{}.Flag(record, "hasRawSource")
	email.Attachments = EmailRecordUtils{}.attachments(RecordUtils{}.Records(record, "attachments"))
	email.GithubUrl = GitHubNotificationUtils{}.SafeTarget(RecordUtils{}.Text(record, "githubUrl"))
	email.UnsubscribeUrl = RecordUtils{}.Text(record, "unsubscribeUrl")
	email.UnsubscribeOneClick = RecordUtils{}.Flag(record, "unsubscribeOneClick")
	email.UnsubscribeMailto = RecordUtils{}.Text(record, "unsubscribeMailto")
	email.UnsubscribeBodyUrl = RecordUtils{}.Text(record, "unsubscribeBodyUrl")
}

func (EmailRecordUtils) openedKey() string { return "openedAt" }

func (EmailRecordUtils) scoreKey() string { return "spamScore" }

func (EmailRecordUtils) addState(email *Email, record map[string]any) {
	email.Folder = RecordUtils{}.Text(record, "folder")
	email.SystemFolder = RecordUtils{}.Text(record, "systemFolder")
	email.Domain = RecordUtils{}.Text(record, "domain")
	email.Labels = RecordUtils{}.Texts(record, "labels")
	email.Read = RecordUtils{}.Flag(record, "read")
	email.Starred = RecordUtils{}.Flag(record, "starred")
	email.ReceivedAt = RecordUtils{}.Text(record, "receivedAt")
	email.OpenedAt = RecordUtils{}.Text(record, EmailRecordUtils{}.openedKey())
	email.HasSpamScore = RecordUtils{}.HasNumber(record, EmailRecordUtils{}.scoreKey())
	email.SpamScoreLabel = strconv.FormatFloat(RecordUtils{}.Number(record, EmailRecordUtils{}.scoreKey()), 'f', 1, 64)
	email.Spam = EmailRecordUtils{}.spam(TextUtils{}.Or(TextUtils{}.Or(email.SystemFolder, email.Folder), "inbox"), email.HasSpamScore, RecordUtils{}.IsList(record, "spamReasons"))
	email.ReplyToMismatch = slices.Contains(RecordUtils{}.Texts(record, "securityWarnings"), "reply_to_mismatch")
}

func (EmailRecordUtils) spam(mailbox string, hasScore bool, hasReasons bool) bool {
	if mailbox == "spam" || hasScore || hasReasons {
		return true
	}
	return false
}

func (EmailRecordUtils) opensKey() string { return "opens" }

func (EmailRecordUtils) clicksKey() string { return "clicks" }

func (EmailRecordUtils) addTracking(email *Email, record map[string]any) {
	opens := RecordUtils{}.Records(record, EmailRecordUtils{}.opensKey())
	clicks := RecordUtils{}.Records(record, EmailRecordUtils{}.clicksKey())
	email.OpenTracking = RecordUtils{}.Flag(record, "openTracking")
	email.LinkTracking = RecordUtils{}.Flag(record, "linkTracking")
	email.TrackingEvents = EmailRecordUtils{}.trackingEvents(EmailRecordUtils{}.openEvents(opens), EmailRecordUtils{}.clickEvents(clicks))
	openCount := max(len(opens), int(max(0, RecordUtils{}.Number(record, "openCount"))))
	clickCount := max(len(clicks), int(max(0, RecordUtils{}.Number(record, "clickCount"))))
	email.TrackingSummary = EmailRecordUtils{}.trackingSummary(openCount, clickCount)
}

func (EmailRecordUtils) trackingEvents(opens []TrackingEvent, clicks []TrackingEvent) []TrackingEvent {
	events := append(opens, clicks...)
	sort.SliceStable(events, func(left int, right int) bool { return events[left].Timestamp > events[right].Timestamp })
	return events
}

func (EmailRecordUtils) openType() string { return "open" }

func (EmailRecordUtils) openEvents(opens []map[string]any) []TrackingEvent {
	events := []TrackingEvent{}
	for _, open := range opens {
		events = append(events, TrackingEvent{Type: EmailRecordUtils{}.openType(), EventIcon: "◉", EventLabel: "Opened", Timestamp: RecordUtils{}.Text(open, EmailRecordUtils{}.openedKey()), RecipientLabel: EmailRecordUtils{}.recipientLabel(open)})
	}
	return events
}

func (EmailRecordUtils) clickType() string { return "click" }

func (EmailRecordUtils) clickEvents(clicks []map[string]any) []TrackingEvent {
	events := []TrackingEvent{}
	for _, click := range clicks {
		events = append(events, TrackingEvent{Type: EmailRecordUtils{}.clickType(), EventIcon: "↗", EventLabel: "Clicked", Timestamp: RecordUtils{}.Text(click, "clickedAt"), RecipientLabel: EmailRecordUtils{}.recipientLabel(click), TargetLabel: EmailRecordUtils{}.targetLabel(click)})
	}
	return events
}

func (EmailRecordUtils) recipientLabel(event map[string]any) string {
	return TextUtils{}.Or(RecordUtils{}.Text(event, EmailRecordUtils{}.recipientKey()), "Recipient unavailable")
}

func (EmailRecordUtils) titleKey() string { return "title" }

func (EmailRecordUtils) targetLabel(event map[string]any) string {
	return TextUtils{}.Or(TextUtils{}.Or(RecordUtils{}.Text(event, EmailRecordUtils{}.titleKey()), RecordUtils{}.Text(event, "url")), "Link")
}

func (EmailRecordUtils) trackingSummary(opens int, clicks int) string {
	if opens == 0 && clicks == 0 {
		return "No activity yet"
	}
	return TextUtils{}.Count(opens, EmailRecordUtils{}.openType(), EmailRecordUtils{}.opensKey()) + " · " + TextUtils{}.Count(clicks, EmailRecordUtils{}.clickType(), EmailRecordUtils{}.clicksKey())
}

func (EmailRecordUtils) attachments(records []map[string]any) []EmailAttachment {
	attachments := []EmailAttachment{}
	for _, item := range records {
		attachments = append(attachments, EmailRecordUtils{}.attachment(item))
	}
	return attachments
}

func (EmailRecordUtils) attachment(record map[string]any) EmailAttachment {
	contentType := RecordUtils{}.Text(record, "contentType")
	return EmailAttachment{
		Id:          RecordUtils{}.Text(record, EmailRecordUtils{}.idKey()),
		Filename:    RecordUtils{}.Text(record, "filename"),
		Size:        RecordUtils{}.Number(record, "size"),
		ContentType: contentType,
		ContentId:   RecordUtils{}.Text(record, "contentId"),
		Disposition: RecordUtils{}.Text(record, "disposition"),
		PreviewKind: AttachmentContentUtils{}.PreviewKind(contentType),
	}
}

func (EmailRecordUtils) statusKey() string { return "status" }

func (EmailRecordUtils) invite(record map[string]any) *CalendarInvite {
	if record == nil {
		return nil
	}
	invite := &CalendarInvite{
		Id:           RecordUtils{}.Text(record, EmailRecordUtils{}.idKey()),
		Title:        RecordUtils{}.Text(record, EmailRecordUtils{}.titleKey()),
		Start:        RecordUtils{}.Text(record, "start"),
		End:          RecordUtils{}.Text(record, "end"),
		AllDay:       RecordUtils{}.Flag(record, "allDay"),
		Location:     RecordUtils{}.Text(record, "location"),
		Organizer:    RecordUtils{}.Text(record, "organizer"),
		Status:       RecordUtils{}.Text(record, EmailRecordUtils{}.statusKey()),
		MyStatus:     RecordUtils{}.Text(record, "myStatus"),
		CanRespond:   RecordUtils{}.Flag(record, "canRespond"),
		IsRecurring:  RecordUtils{}.Flag(record, "isRecurring"),
		RecurrenceId: RecordUtils{}.Text(record, "recurrenceId"),
	}
	invite.SenderResponse = EmailRecordUtils{}.response(RecordUtils{}.Record(record, "senderResponse"))
	invite.Attachment = EmailRecordUtils{}.inviteAttachment(RecordUtils{}.Record(record, "attachment"))
	return invite
}

func (EmailRecordUtils) response(record map[string]any) *InviteResponse {
	if record == nil {
		return nil
	}
	return &InviteResponse{Status: RecordUtils{}.Text(record, EmailRecordUtils{}.statusKey()), Name: RecordUtils{}.Text(record, "name"), Email: RecordUtils{}.Text(record, "email")}
}

func (EmailRecordUtils) inviteAttachment(record map[string]any) *EmailAttachment {
	if record == nil {
		return nil
	}
	attachment := EmailRecordUtils{}.attachment(record)
	return &attachment
}
