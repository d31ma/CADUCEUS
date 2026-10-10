import (
	"encoding/base64"
	"strings"
)

// ScheduledSends manages scheduled messages for their exact owner: lookup,
// the bounded management view, cancellation (queue work first, so a deleted
// record can never still send) and the stored attachment set.
type ScheduledSends struct {
	Sent      SentMail
	Reminders FollowUpReminders
}

// FindOwned answers the owner's scheduled message with the id, or nil.
func (schedules ScheduledSends) FindOwned(id string, ownerEmail string) (*ScheduledRecord, error) {
	documentId, entry, err := schedules.Sent.Scheduled.FindById(id)
	if err != nil || documentId == "" || entry == nil || schedules.owner(entry) != ownerEmail {
		return nil, err
	}
	record := &ScheduledRecord{DocumentId: documentId, Entry: entry}
	if entry["transport"] == "external" {
		return record, nil
	}
	values := RecordValues{}
	record.QueueDocumentId, record.Queue, err = schedules.Sent.Outbox.FindById(values.Text(values.Coalesce(entry["queueId"], entry["id"])))
	return record, err
}

// Present answers the management view of a scheduled message; attachment
// bytes are included only for editing. submission is the provider journal
// entry of a connected-account schedule, or nil.
func (schedules ScheduledSends) Present(record *ScheduledRecord, includeContent bool, submission map[string]any) (map[string]any, error) {
	entry, queue := record.Entry, record.Queue
	values := RecordValues{}
	email, _ := values.Object(entry["email"])
	id := values.Text(entry["id"])
	reminder, err := schedules.Reminders.Reminders.FindActiveForEmail(schedules.owner(entry), id)
	if err != nil {
		return nil, err
	}
	attachments, err := schedules.StoredAttachments(id, includeContent)
	if err != nil {
		return nil, err
	}
	sendAt, _ := DeliveryFields{}.Millis(entry["sendAt"])
	createdAt, _ := DeliveryFields{}.Millis(entry["createdAt"])
	presented := map[string]any{
		"id": entry["id"], "from": entry["sender"], "to": email["to"],
		"cc": values.Coalesce(email["cc"], []any{}), "bcc": values.Coalesce(email["bcc"], []any{}), "replyTo": values.Coalesce(email["replyTo"], []any{}),
		"subject": email["subject"], "text": values.Coalesce(email["text"], ""), "html": values.Coalesce(email["html"], ""),
		"inReplyTo": values.Coalesce(email["inReplyTo"], ""), "references": values.Coalesce(email["references"], []any{}),
		"openTracking": email["openTracking"] == true, "linkTracking": email["linkTracking"] == true, "attachments": attachments,
		"sendAt": EpochTime{}.Text(sendAt), "createdAt": EpochTime{}.Text(createdAt),
		"status": schedules.status(entry, queue, submission), "remindAt": nil,
	}
	if values.Truthy(entry["updatedAt"]) {
		updatedAt, _ := DeliveryFields{}.Millis(entry["updatedAt"])
		presented["updatedAt"] = EpochTime{}.Text(updatedAt)
	}
	presented["attempts"] = values.Coalesce(submission["attempts"], values.Coalesce(queue["attempts"], 0))
	presented["nextAttempt"] = schedules.nextAttempt(queue, submission)
	presented["lastError"] = TextValue{}.Prefix(values.Text(values.Or(submission["lastCode"], values.Or(queue["lastError"], ""))), 500)
	if reminder != nil {
		remindAt, _ := DeliveryFields{}.Millis(reminder["remindAt"])
		presented["remindAt"] = EpochTime{}.Text(remindAt)
	}
	return presented, nil
}

func (ScheduledSends) status(entry map[string]any, queue map[string]any, submission map[string]any) any {
	if entry["transport"] == "external" {
		if state := submission["state"]; state == "failed" || state == "ambiguous" {
			return state
		}
		return "scheduled"
	}
	if queue != nil && queue["status"] != nil {
		return queue["status"]
	}
	if entry["status"] == "scheduled" || !(RecordValues{}).Truthy(entry["status"]) {
		return "reconciling"
	}
	return entry["status"]
}

func (ScheduledSends) nextAttempt(queue map[string]any, submission map[string]any) any {
	if submission["notBefore"] != nil {
		return submission["notBefore"]
	}
	if queue == nil {
		return nil
	}
	at, _ := DeliveryFields{}.Millis(RecordValues{}.Coalesce(queue["nextAttempt"], queue["expiresAt"]))
	return EpochTime{}.Text(at)
}

// StoredAttachments answers one message's bucket-stored attachments.
func (schedules ScheduledSends) StoredAttachments(emailId string, includeContent bool) ([]any, error) {
	records, err := schedules.Sent.Attachments.List(emailId)
	if err != nil {
		return nil, err
	}
	values := RecordValues{}
	attachments := []any{}
	for _, record := range records {
		attachment := map[string]any{
			"filename": record["filename"], "contentType": record["contentType"], "size": record["size"],
			"disposition": values.Coalesce(record["disposition"], "attachment"),
		}
		if contentId := values.Text(record["contentId"]); contentId != "" {
			attachment["contentId"] = contentId
		}
		if includeContent {
			content, err := schedules.Sent.Attachments.ReadContent(record)
			if err != nil {
				return nil, err
			}
			if len(content) > 0 {
				attachment["contentBase64"] = base64.StdEncoding.EncodeToString(content)
			}
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// Cancel cancels a hosted schedule unless a delivery lease holds it, and
// answers {cancelled} or {conflict}.
func (schedules ScheduledSends) Cancel(record *ScheduledRecord, ownerEmail string) (map[string]any, error) {
	now := EpochTime{}.Now()
	claimedAt := RecordValues{}.NumberOf(record.Queue["claimedAt"])
	if record.Queue != nil && record.Queue["status"] == "sending" && claimedAt > float64(now-(OutboundQueue{}).lease()) {
		return map[string]any{"conflict": true}, nil
	}
	if record.QueueDocumentId != "" {
		if err := schedules.Sent.Outbox.Delete(record.QueueDocumentId); err != nil {
			return nil, err
		}
	}
	id := RecordValues{}.Text(record.Entry["id"])
	if err := schedules.retire(record, ownerEmail); err != nil {
		return nil, err
	}
	emailDocumentId, _, err := schedules.Sent.Emails.FindById(id)
	if err == nil && emailDocumentId != "" {
		err = schedules.Sent.Emails.Delete(emailDocumentId, id)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"cancelled": true}, nil
}

// CancelExternal cancels the provider journal entry before deleting its
// local management copy.
func (schedules ScheduledSends) CancelExternal(record *ScheduledRecord, ownerEmail string, token string) (map[string]any, error) {
	values := RecordValues{}
	accountId, submissionId := values.Text(record.Entry["externalAccountId"]), values.Text(record.Entry["externalSubmissionId"])
	if record.Entry["transport"] != "external" || accountId == "" || submissionId == "" {
		return map[string]any{"conflict": true}, nil
	}
	if _, err := (ProviderAccounts{Token: token}).Call("accounts.cancel_submission", accountId, map[string]any{"submissionId": submissionId}); err != nil {
		return nil, err
	}
	if err := schedules.retire(record, ownerEmail); err != nil {
		return nil, err
	}
	if err := schedules.Sent.RollbackExternalAccepted(values.Text(record.Entry["id"])); err != nil {
		return nil, err
	}
	return map[string]any{"cancelled": true}, nil
}

func (schedules ScheduledSends) retire(record *ScheduledRecord, ownerEmail string) error {
	if err := schedules.Sent.Scheduled.Delete(record.DocumentId); err != nil {
		return err
	}
	_, err := schedules.Reminders.DismissForEmail(RecordValues{}.Text(record.Entry["id"]), ownerEmail)
	return err
}

// Submission answers the provider journal entry of a connected-account
// schedule.
func (ScheduledSends) Submission(record *ScheduledRecord, token string) (map[string]any, error) {
	values := RecordValues{}
	return ProviderAccounts{Token: token}.Call("accounts.submission", values.Text(record.Entry["externalAccountId"]), map[string]any{"submissionId": values.Text(record.Entry["externalSubmissionId"])})
}

func (ScheduledSends) owner(entry map[string]any) string {
	values := RecordValues{}
	return strings.TrimSpace(values.Text(values.Coalesce(entry["owner"], entry["sender"])))
}
