import (
	"encoding/base64"
	"math"
	"strings"
	"time"
)

// ScheduledEditing answers PUT /send/scheduled/{id}: an in-place edit of a
// pending scheduled message that keeps its canonical id, for hosted
// delivery or for the caller's connected-account journal.
type ScheduledEditing struct {
	Schedules ScheduledSends
	Senders   SenderAddresses
}

// Edit answers the route answer of one edit.
func (editing ScheduledEditing) Edit(record *ScheduledRecord, caller RouteCaller, body map[string]any) (any, error) {
	if body == nil {
		body = map[string]any{}
	}
	values := RecordValues{}
	entry := record.Entry
	external := entry["transport"] == "external"
	if !external && (record.QueueDocumentId == "" || record.Queue == nil) {
		return map[string]any{"error": "Scheduled delivery is being reconciled", "conflict": true}, nil
	}
	if !external && record.Queue["status"] == "sending" {
		return map[string]any{"error": "Scheduled email is currently being delivered", "conflict": true}, nil
	}
	stored, _ := values.Object(entry["email"])
	parsed, problem := MailValidation{}.SendRequest(DeliveryFields{}.Merged(stored, body))
	if problem != "" {
		return map[string]any{"error": problem}, nil
	}
	attachmentsInput := body["attachments"]
	if !(DeliveryFields{}).Has(body, "attachments") {
		storedAttachments, err := editing.Schedules.StoredAttachments(values.Text(entry["id"]), true)
		if err != nil {
			return nil, err
		}
		attachmentsInput = storedAttachments
	}
	attachments, problem := MailValidation{}.Attachments(attachmentsInput)
	if problem != "" {
		return map[string]any{"error": problem}, nil
	}
	sender, err := editing.sender(external, parsed, entry, caller)
	if err != nil {
		return nil, err
	}
	if sender == "" {
		return map[string]any{"error": "From address is not owned by this mailbox", "forbidden": true}, nil
	}
	timing, answer, err := editing.timing(record, caller, body)
	if answer != nil || err != nil {
		return answer, err
	}
	request := DeliveryFields{}.Merged(parsed, map[string]any{"messageId": stored["messageId"]})
	if len(attachments) > 0 {
		request["attachments"] = attachments
	}
	suppressed, err := editing.Schedules.Sent.Queue.Suppressed.AsSet()
	if err != nil {
		return nil, err
	}
	blocked := []string{}
	fields := DeliveryFields{}
	for _, address := range append(append(fields.Strings(request["to"]), fields.Strings(request["cc"])...), fields.Strings(request["bcc"])...) {
		if suppressed[address] {
			blocked = append(blocked, address)
		}
	}
	if len(blocked) > 0 {
		return map[string]any{"error": "Recipients are suppressed", "blocked": blocked}, nil
	}
	edit := ScheduledEdit{ScheduledDocumentId: record.DocumentId, Entry: entry, QueueDocumentId: record.QueueDocumentId, Owner: caller.Email, Sender: sender, Request: request, SendAt: timing["sendAt"]}
	var result map[string]any
	if external {
		answer, result, err = editing.external(record, caller, edit)
		if answer != nil || err != nil {
			return answer, err
		}
	} else if result, err = editing.Schedules.Sent.UpdateScheduled(edit); err != nil {
		return nil, err
	}
	return result, editing.remind(record, caller, body, request, result, timing)
}

func (editing ScheduledEditing) sender(external bool, parsed map[string]any, entry map[string]any, caller RouteCaller) (string, error) {
	requested := RecordValues{}.Coalesce(parsed["from"], entry["sender"])
	if external {
		if requested != entry["sender"] {
			return "", nil
		}
		return RecordValues{}.Text(requested), nil
	}
	return editing.Senders.Sender(caller.Email, requested)
}

// timing answers the new sendAt and remindAt ("remindAt" -1 for none), or
// the refusal of an unusable time.
func (editing ScheduledEditing) timing(record *ScheduledRecord, caller RouteCaller, body map[string]any) (map[string]int64, any, error) {
	sendAt := RecordValues{}.Numeric(record.Entry["sendAt"])
	if body["sendAt"] != nil {
		sendAt = EpochTime{}.Parse(RecordValues{}.String(body["sendAt"]))
	}
	if math.IsNaN(sendAt) || math.IsInf(sendAt, 0) {
		return nil, map[string]any{"error": "sendAt must be a valid ISO datetime"}, nil
	}
	if sendAt <= float64(EpochTime{}.Now()) {
		return nil, map[string]any{"error": "sendAt must be in the future"}, nil
	}
	active, err := editing.Schedules.Reminders.Reminders.FindActiveForEmail(caller.Email, RecordValues{}.Text(record.Entry["id"]))
	if err != nil {
		return nil, nil, err
	}
	remindAt := -1.0
	if active != nil {
		remindAt = RecordValues{}.Numeric(active["remindAt"])
	}
	cleared := (DeliveryFields{}).Has(body, "remindAt") && body["remindAt"] == nil
	if cleared {
		remindAt = -1
	} else if body["remindAt"] != nil {
		text, isText := body["remindAt"].(string)
		if !isText {
			return nil, map[string]any{"error": "remindAt must be an ISO datetime or null"}, nil
		}
		if remindAt = (EpochTime{}).Parse(text); math.IsNaN(remindAt) {
			return nil, map[string]any{"error": "remindAt must be a valid ISO datetime"}, nil
		}
	}
	if remindAt != -1 && remindAt <= sendAt {
		return nil, map[string]any{"error": "remindAt must be after sendAt"}, nil
	}
	return map[string]int64{"sendAt": int64(sendAt), "remindAt": int64(remindAt)}, nil, nil
}

func (editing ScheduledEditing) remind(record *ScheduledRecord, caller RouteCaller, body map[string]any, request map[string]any, result map[string]any, timing map[string]int64) error {
	id := RecordValues{}.Text(record.Entry["id"])
	reminders := editing.Schedules.Reminders
	if (DeliveryFields{}).Has(body, "remindAt") && body["remindAt"] == nil {
		_, err := reminders.DismissForEmail(id, caller.Email)
		return err
	}
	if timing["remindAt"] == -1 {
		return nil
	}
	_, err := reminders.Schedule(map[string]any{
		"ownerEmail": caller.Email, "emailId": id, "recipient": strings.Join(DeliveryFields{}.Strings(request["to"]), ", "),
		"subject": request["subject"], "threadId": RecordValues{}.Text(result["threadId"]), "remindAt": timing["remindAt"],
	})
	return err
}

// external replaces a connected-account schedule in the provider journal
// under its exact revision, then the local copies.
func (editing ScheduledEditing) external(record *ScheduledRecord, caller RouteCaller, edit ScheduledEdit) (any, map[string]any, error) {
	values := RecordValues{}
	accountId, submissionId := values.Text(record.Entry["externalAccountId"]), values.Text(record.Entry["externalSubmissionId"])
	if accountId == "" || submissionId == "" {
		return map[string]any{"error": "Scheduled delivery is being reconciled", "conflict": true}, nil, nil
	}
	accounts := ProviderAccounts{Token: caller.Token}
	submission, err := editing.Schedules.Submission(record, caller.Token)
	if err != nil {
		return accounts.Failure(err), nil, nil
	}
	revision, isNumber := values.Number(submission["revision"])
	if submission["state"] != "queued" || !isNumber || revision != math.Trunc(revision) || revision < 1 {
		return map[string]any{"error": "Scheduled email is currently being delivered", "conflict": true}, nil, nil
	}
	request := edit.Request
	recipients := ProviderSend{}.unique(OutboundQueue{}.Requested(request))
	stored, _ := values.Object(record.Entry["email"])
	trackingId := values.Text(stored["trackingId"])
	tracked := request["openTracking"] == true || request["linkTracking"] == true
	if tracked && trackingId == "" {
		trackingId = DeliveryFields{}.UuidSeven()
	}
	if !tracked {
		trackingId = ""
	}
	id := values.Text(record.Entry["id"])
	notBefore := EpochTime{}.Text(edit.SendAt)
	if values.NumberOf(submission["deliveryCount"]) > 0 {
		err = editing.batch(record, accounts, submission, recipients, trackingId, edit, notBefore)
	} else {
		if trackingId != "" && len(recipients) != 1 {
			return map[string]any{"error": "This legacy scheduled message must be recreated to track multiple recipients", "conflict": true}, nil, nil
		}
		var delivery map[string]any
		if delivery, err = (ExternalSubmission{}).DeliveryRequest(request, trackingId, recipients[0]); err == nil {
			source := MimeMessage{}.Source(edit.Sender, delivery, id, time.Now())
			_, err = accounts.Call("accounts.update_submission", accountId, map[string]any{
				"submissionId": submissionId, "revision": submission["revision"], "idempotencyKey": submission["idempotencyKey"],
				"envelopeFrom": edit.Sender, "recipients": recipients, "rawRFC822Base64": base64.StdEncoding.EncodeToString(source), "notBefore": notBefore,
			})
		}
	}
	if err != nil {
		return accounts.Failure(err), nil, nil
	}
	result, err := editing.Schedules.Sent.UpdateExternalScheduled(edit, trackingId)
	if err != nil {
		return accounts.Failure(err), nil, nil
	}
	return nil, result, nil
}

// batch stages, uploads and commits a tracked multi-recipient edit; a
// failure aborts a staged edit that is still open.
func (editing ScheduledEditing) batch(record *ScheduledRecord, accounts ProviderAccounts, submission map[string]any, recipients []string, trackingId string, edit ScheduledEdit, notBefore string) error {
	values := RecordValues{}
	accountId, submissionId := values.Text(record.Entry["externalAccountId"]), values.Text(record.Entry["externalSubmissionId"])
	staged, err := accounts.Call("accounts.submit_batch_update_begin", accountId, map[string]any{
		"submissionId": submissionId, "revision": submission["revision"], "idempotencyKey": submission["idempotencyKey"],
		"envelopeFrom": edit.Sender, "recipients": recipients, "notBefore": notBefore,
	})
	if err == nil {
		_, err = ProviderSend{}.Batch(BatchUpload{
			Accounts: accounts, AccountId: accountId, Operation: "accounts.submit_batch_update", Submission: staged, SubmissionId: submissionId,
			Request: edit.Request, TrackingId: trackingId, Sender: edit.Sender, MessageId: values.Text(record.Entry["id"]), Recipients: recipients,
		})
	}
	if err != nil {
		if current, currentErr := editing.Schedules.Submission(record, accounts.Token); currentErr == nil && current["state"] == "editing" {
			_, _ = accounts.Call("accounts.submit_batch_update_abort", accountId, map[string]any{"submissionId": submissionId, "revision": current["revision"]})
		}
	}
	return err
}
