import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"
)

// SentMail accepts outbound mail: it delivers now, delays with an undo
// window, or schedules, and keeps the Sent copy (its row, MIME source and
// attachment files) that tracking and the Sent folder read. A request is the
// validated send fields plus messageId and attachments.
type SentMail struct {
	Emails       *EmailsHelper
	Attachments  *AttachmentsHelper
	RawMessages  *RawMessagesHelper
	Scheduled    *ScheduledHelper
	Outbox       *OutboxHelper
	Queue        OutboundQueue
	Participants *ContactsService
	Users        *UsersHelper
}

// SendNow stores the Sent copy, then delivers. A delivery hiccup does not
// fail the request: the message queues for retry a minute later.
func (sent SentMail) SendNow(ownerEmail string, senderEmail string, request map[string]any, trackingId string) (map[string]any, error) {
	messageId := sent.messageId(request, "sent-")
	delivery := sent.delivery(request, trackingId)
	stored, err := sent.store(trackingId, ownerEmail, senderEmail, messageId, request)
	if err != nil {
		return nil, err
	}
	if _, err := sent.Queue.Smtp.SendEmail(senderEmail, delivery); err != nil {
		log.Printf("[send] immediate delivery failed, queuing for retry: %v", err)
		var failure *SmtpFailure
		retry := delivery
		if errors.As(err, &failure) && len(failure.Failed) > 0 {
			retry = DeliveryFields{}.Merged(delivery, map[string]any{"envelopeRecipients": DeliveryFields{}.Items(failure.Failed)})
		}
		queueId, err := sent.Queue.Enqueue(senderEmail, sent.queued(retry), map[string]any{
			"owner": ownerEmail, "availableAt": EpochTime{}.Now() + 60_000, "trackingId": trackingId, "attachmentEmailId": messageId,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"messageId": messageId, "queueId": queueId, "threadId": stored["threadId"], "queued": true}, nil
	}
	return map[string]any{"messageId": messageId, "threadId": stored["threadId"]}, nil
}

// RecordExternalAccepted stores the Sent copy before a connected-account
// submission is made runnable.
func (sent SentMail) RecordExternalAccepted(ownerEmail string, senderEmail string, request map[string]any, trackingId string) (map[string]any, error) {
	messageId := sent.messageId(request, "sent-")
	stored, err := sent.store(trackingId, ownerEmail, senderEmail, messageId, request)
	if err != nil {
		return nil, err
	}
	return map[string]any{"messageId": messageId, "threadId": stored["threadId"]}, nil
}

// RollbackExternalAccepted removes a Sent copy whose submission was undone.
func (sent SentMail) RollbackExternalAccepted(messageId string) error {
	documentId, _, err := sent.Emails.FindById(messageId)
	if err != nil {
		return err
	}
	if err := sent.Attachments.DeleteForEmail(messageId); err != nil {
		return err
	}
	if err := sent.RawMessages.DeleteForEmail(messageId); err != nil {
		return err
	}
	if documentId == "" {
		return nil
	}
	return sent.Emails.Delete(documentId, messageId)
}

// Delay queues the message behind an undo window; its outbox id is the undo
// token. Delivery happens on the next worker pass after the window (the
// JavaScript server also started an in-process pass then; a Tachyon request
// process ends with its response).
func (sent SentMail) Delay(ownerEmail string, senderEmail string, request map[string]any, delayMs int64, trackingId string) (map[string]any, error) {
	now := EpochTime{}.Now()
	undoId := "undo-" + strconv.FormatInt(now, 36) + "-" + DeliveryFields{}.RandomBase36(8)
	availableAt := now + delayMs
	entry := map[string]any{
		"id": undoId, "email": sent.queued(sent.delivery(request, trackingId)), "sender": senderEmail, "owner": ownerEmail,
		"expiresAt": availableAt, "nextAttempt": availableAt, "attempts": 0, "status": "queued", "createdAt": now,
		"attachmentEmailId": undoId,
	}
	if trackingId != "" {
		entry["trackingId"] = trackingId
	}
	if _, err := sent.Outbox.Put(entry); err != nil {
		return nil, err
	}
	stored, err := sent.store(trackingId, ownerEmail, senderEmail, undoId, request)
	if err != nil {
		return nil, err
	}
	return map[string]any{"sent": true, "undoId": undoId, "threadId": stored["threadId"]}, nil
}

// Schedule queues the message for an absolute time and keeps its management
// record; either both exist or neither does.
func (sent SentMail) Schedule(ownerEmail string, senderEmail string, request map[string]any, sendAtMs int64, trackingId string) (map[string]any, error) {
	now := EpochTime{}.Now()
	scheduledId := "scheduled-" + strconv.FormatInt(now, 36) + "-" + DeliveryFields{}.RandomBase36(8)
	delivery := sent.queued(sent.delivery(request, trackingId))
	scheduledDocumentId, err := sent.Scheduled.Put(map[string]any{
		"id": scheduledId, "email": delivery, "sender": senderEmail, "owner": ownerEmail,
		"sendAt": sendAtMs, "createdAt": now, "status": "scheduled",
	})
	if err != nil {
		return nil, err
	}
	queueId, err := sent.Queue.Enqueue(senderEmail, delivery, map[string]any{
		"owner": ownerEmail, "availableAt": sendAtMs, "trackingId": trackingId, "id": scheduledId,
		"scheduledId": scheduledId, "attachmentEmailId": scheduledId,
	})
	if err == nil {
		err = sent.Scheduled.Update(scheduledDocumentId, map[string]any{"queueId": queueId})
	}
	if err != nil {
		_ = sent.Scheduled.Delete(scheduledDocumentId)
		return nil, err
	}
	stored, err := sent.store(trackingId, ownerEmail, senderEmail, scheduledId, request)
	if err != nil {
		if queueDocumentId, _, _ := sent.Outbox.FindById(queueId); queueDocumentId != "" {
			_ = sent.Outbox.Delete(queueDocumentId)
		}
		_ = sent.Scheduled.Delete(scheduledDocumentId)
		return nil, err
	}
	return map[string]any{"scheduled": true, "id": scheduledId, "threadId": stored["threadId"]}, nil
}

// UpdateScheduled replaces a pending scheduled message in place, keeping its
// canonical id.
func (sent SentMail) UpdateScheduled(edit ScheduledEdit) (map[string]any, error) {
	email, _ := RecordValues{}.Object(edit.Entry["email"])
	trackingId := RecordValues{}.Text(email["trackingId"])
	id := RecordValues{}.Text(edit.Entry["id"])
	delivery := sent.queued(sent.delivery(edit.Request, trackingId))
	err := sent.Outbox.Update(edit.QueueDocumentId, map[string]any{
		"email": delivery, "sender": edit.Sender, "owner": edit.Owner, "expiresAt": edit.SendAt, "nextAttempt": edit.SendAt,
		"attempts": 0, "status": "queued", "lastError": "", "claimedAt": 0, "attachmentEmailId": id,
	})
	if err != nil {
		return nil, err
	}
	return sent.replaceScheduled(edit, delivery, trackingId)
}

// UpdateExternalScheduled replaces the local management and Sent copies
// after the Go-owned provider journal accepted an in-place edit.
func (sent SentMail) UpdateExternalScheduled(edit ScheduledEdit, trackingId string) (map[string]any, error) {
	return sent.replaceScheduled(edit, sent.queued(sent.delivery(edit.Request, trackingId)), trackingId)
}

func (sent SentMail) replaceScheduled(edit ScheduledEdit, delivery map[string]any, trackingId string) (map[string]any, error) {
	err := sent.Scheduled.Update(edit.ScheduledDocumentId, map[string]any{
		"email": delivery, "sender": edit.Sender, "sendAt": edit.SendAt, "updatedAt": EpochTime{}.Now(), "status": "scheduled",
	})
	if err != nil {
		return nil, err
	}
	id := RecordValues{}.Text(edit.Entry["id"])
	stored, err := sent.replace(trackingId, edit.Owner, edit.Sender, id, edit.Request)
	if err != nil {
		return nil, err
	}
	return map[string]any{"scheduled": true, "id": id, "threadId": stored["threadId"], "sendAt": EpochTime{}.Text(edit.SendAt)}, nil
}

// AttachArchiveRollback stores the source-archive undo state on a delayed
// send before the route moves any messages.
func (sent SentMail) AttachArchiveRollback(undoId string, archiveStateJson string) error {
	documentId, entry, err := sent.Outbox.FindById(undoId)
	if err != nil {
		return err
	}
	if documentId == "" || entry == nil {
		return errors.New("Undo delivery is no longer available")
	}
	return sent.Outbox.Update(documentId, map[string]any{"archiveStateJson": archiveStateJson})
}

// replace updates the Sent row, MIME source and attachment set.
func (sent SentMail) replace(trackingId string, ownerEmail string, senderEmail string, messageId string, request map[string]any) (map[string]any, error) {
	documentId, _, err := sent.Emails.FindById(messageId)
	if err != nil {
		return nil, err
	}
	if documentId == "" {
		return sent.store(trackingId, ownerEmail, senderEmail, messageId, request)
	}
	fields := DeliveryFields{}
	domain := fields.Domain(ownerEmail)
	metadata := Threading{}.ThreadMetadataFromOutbound(request, messageId, domain)
	html := RecordValues{}.Text(request["html"])
	links := []any{}
	if request["linkTracking"] == true {
		links = TrackedHtml{}.TrackableLinks(html)
	}
	patch := fields.Merged(map[string]any{
		"ownerEmail": ownerEmail, "recipient": strings.Join(fields.Strings(request["to"]), ", "), "sender": senderEmail,
		"replyTo": sent.list(request["replyTo"]), "to": sent.lowered(request["to"]), "cc": sent.lowered(request["cc"]), "bcc": sent.lowered(request["bcc"]),
		"subject": request["subject"], "body": sent.plainBody(request), "htmlBody": html,
		"openTracking": request["openTracking"] == true, "linkTracking": request["linkTracking"] == true,
		"trackedLinksJson": StoredRecords{}.JsonText(links),
	}, metadata)
	if err := sent.Emails.Update(documentId, patch); err != nil {
		return nil, err
	}
	if _, err := sent.Attachments.ReplaceForEmail(messageId, domain, sent.parsedAttachments(request)); err != nil {
		return nil, err
	}
	if err := sent.saveSource(senderEmail, request, messageId, domain); err != nil {
		return nil, err
	}
	sent.observe(ownerEmail, senderEmail, request)
	return metadata, nil
}

// store keeps a Sent copy, so the tracking pixel and click redirects have a
// row to update; it is written before delivery so a fast recipient can never
// present a capability before its Sent row exists.
func (sent SentMail) store(trackingId string, ownerEmail string, senderEmail string, messageId string, request map[string]any) (map[string]any, error) {
	fields := DeliveryFields{}
	domain := fields.Domain(ownerEmail)
	metadata := Threading{}.ThreadMetadataFromOutbound(request, messageId, domain)
	email := map[string]any{
		"id": messageId, "domain": domain, "ownerEmail": ownerEmail,
		"recipient": strings.Join(fields.Strings(request["to"]), ", "), "sender": senderEmail,
		"subject": request["subject"], "body": sent.plainBody(request),
		"folder":  CategoryMappings{}.OfOwner(sent.Users, ownerEmail).PathFor(domain, "sent"),
		"read":    true, "starred": false, "receivedAt": EpochTime{}.NowText(), "processed": true,
	}
	for _, field := range []string{"replyTo"} {
		if list := fields.Strings(request[field]); len(list) > 0 {
			email[field] = fields.Items(list)
		}
	}
	for _, field := range []string{"to", "cc", "bcc"} {
		if list := fields.Strings(request[field]); len(list) > 0 {
			email[field] = sent.lowered(request[field])
		}
	}
	if html := (RecordValues{}).Text(request["html"]); html != "" {
		email["htmlBody"] = html
	}
	if trackingId != "" {
		links := []any{}
		if request["linkTracking"] == true {
			links = TrackedHtml{}.TrackableLinks(RecordValues{}.Text(request["html"]))
		}
		email = fields.Merged(email, map[string]any{
			"trackingId": trackingId, "openTracking": request["openTracking"] == true, "linkTracking": request["linkTracking"] == true,
			"trackedLinksJson": StoredRecords{}.JsonText(links), "opensJson": "[]", "clicksJson": "[]", "openCount": 0, "clickCount": 0,
		})
	}
	if _, err := sent.Emails.Put(fields.Merged(email, metadata)); err != nil {
		return nil, err
	}
	if attachments := sent.parsedAttachments(request); len(attachments) > 0 {
		if _, err := sent.Attachments.SaveForEmail(messageId, domain, attachments); err != nil {
			return nil, err
		}
	}
	if err := sent.saveSource(senderEmail, request, messageId, domain); err != nil {
		return nil, err
	}
	sent.observe(ownerEmail, senderEmail, request)
	return metadata, nil
}

func (sent SentMail) saveSource(senderEmail string, request map[string]any, messageId string, domain string) error {
	_, err := sent.RawMessages.SaveForEmail(messageId, domain, MimeMessage{}.Source(senderEmail, request, messageId, time.Now()))
	return err
}

func (sent SentMail) observe(ownerEmail string, senderEmail string, request map[string]any) {
	fields := DeliveryFields{}
	addresses := append(fields.Strings(request["to"]), fields.Strings(request["cc"])...)
	sent.Participants.ObserveParticipants(ownerEmail, append(addresses, fields.Strings(request["bcc"])...), fields.Domain(senderEmail))
}

// parsedAttachments answers the request's attachments for the attachment
// bucket.
func (SentMail) parsedAttachments(request map[string]any) []ParsedAttachment {
	values := RecordValues{}
	items, _ := values.List(request["attachments"])
	attachments := []ParsedAttachment{}
	for _, item := range items {
		attachment, _ := values.Object(item)
		attachments = append(attachments, ParsedAttachment{
			Filename: values.Text(attachment["filename"]), ContentType: values.Text(attachment["contentType"]),
			ContentBase64: values.Text(attachment["contentBase64"]), Disposition: values.Text(attachment["disposition"]),
			ContentId: values.Text(attachment["contentId"]),
		})
	}
	return attachments
}

func (SentMail) messageId(request map[string]any, prefix string) string {
	if id := (RecordValues{}).Text(request["messageId"]); id != "" {
		return id
	}
	return prefix + strconv.FormatInt(EpochTime{}.Now(), 36)
}

// delivery answers the request with its tracking id.
func (SentMail) delivery(request map[string]any, trackingId string) map[string]any {
	if trackingId == "" {
		return request
	}
	return DeliveryFields{}.Merged(request, map[string]any{"trackingId": trackingId})
}

// queued answers the request without its attachments: queue rows cannot
// embed them, so they live in the attachment bucket.
func (SentMail) queued(request map[string]any) map[string]any {
	return StoredRecords{}.Without(request, "attachments")
}

func (SentMail) list(value any) []any {
	return DeliveryFields{}.Items(DeliveryFields{}.Strings(value))
}

func (SentMail) lowered(value any) []any {
	lowered := []any{}
	for _, address := range (DeliveryFields{}).Strings(value) {
		lowered = append(lowered, strings.ToLower(address))
	}
	return lowered
}

// plainBody answers the text body, else the HTML body without markup.
func (sent SentMail) plainBody(request map[string]any) string {
	if text := (RecordValues{}).Text(request["text"]); text != "" {
		return text
	}
	return HtmlText{}.Stripped(RecordValues{}.Text(request["html"]))
}

