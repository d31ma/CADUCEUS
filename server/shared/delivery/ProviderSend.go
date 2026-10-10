import (
	"encoding/base64"
	"errors"
	"time"
)

// ProviderSend accepts a send whose From address is one of the caller's
// connected accounts: the Sent copy is stored first, then the Go-owned
// provider journal takes the submission (one MIME source per recipient for
// tracked mail, whose bodies carry recipient-specific capabilities). Any
// failure cancels the submission, removes the Sent copy and leaves the
// idempotency row ambiguous.
type ProviderSend struct {
	Acceptance SendAcceptance
}

// Accept answers the route answer of a connected-account send.
func (provider ProviderSend) Accept(plan SendPlan) (any, error) {
	send := provider.Acceptance
	accounts := DeliveryAccounts{Token: plan.Caller.Token}
	accountId := RecordValues{}.Text(plan.Account["id"])
	stored, submission, err := provider.submit(plan, accounts, accountId)
	if err == nil && plan.HasSendAt {
		email := StoredRecords{}.Without(plan.Request, "attachments")
		if plan.TrackingId != "" {
			email["trackingId"] = plan.TrackingId
		}
		_, err = send.Sent.Scheduled.Put(map[string]any{
			"id": stored["messageId"], "email": email, "sender": plan.Sender, "owner": plan.Caller.Email,
			"sendAt": plan.SendAt, "createdAt": EpochTime{}.Now(), "status": "scheduled", "transport": "external",
			"externalAccountId": accountId, "externalSubmissionId": submission["id"],
		})
	}
	var accepted map[string]any
	if err == nil {
		accepted, err = send.WithReminder(plan, provider.accepted(plan, accountId, stored, submission))
	}
	if err != nil {
		if submissionId := (RecordValues{}).Text(submission["id"]); submissionId != "" {
			_, _ = accounts.Call("accounts.cancel_submission", accountId, map[string]any{"submissionId": submissionId})
		}
		if messageId := (RecordValues{}).Text(stored["messageId"]); messageId != "" {
			_ = send.Sent.RollbackExternalAccepted(messageId)
		}
		send.Journal.MarkAmbiguous(plan.JournalId)
		return accounts.Failure(err), nil
	}
	if plan.ArchiveSourceId != "" {
		// Archiving a connected-account source runs through the journaled
		// mail actions, which mail reading owns.
		accepted = DeliveryFields{}.Merged(accepted, map[string]any{
			"archived": false, "archivedCount": 0, "archiveError": "Connected-account source archive is not available",
		})
	}
	return send.Journal.Accept(plan.JournalId, accepted)
}

// submit stores the Sent copy and submits it to the provider journal.
func (provider ProviderSend) submit(plan SendPlan, accounts DeliveryAccounts, accountId string) (map[string]any, map[string]any, error) {
	stored, err := provider.Acceptance.Sent.RecordExternalAccepted(plan.Caller.Email, plan.Sender, plan.Request, plan.TrackingId)
	if err != nil {
		return nil, nil, err
	}
	messageId := RecordValues{}.Text(plan.Request["messageId"])
	key := plan.IdempotencyKey
	if key == "" {
		key = "message:" + messageId
	}
	recipients := provider.unique(plan.Recipients)
	notBefore := provider.notBefore(plan)
	if plan.TrackingId == "" {
		source := MimeMessage{}.Source(plan.Sender, plan.Request, messageId, time.Now())
		submission, err := accounts.Call("accounts.submit", accountId, map[string]any{
			"idempotencyKey": key, "envelopeFrom": plan.Sender, "recipients": recipients,
			"rawRFC822Base64": base64.StdEncoding.EncodeToString(source), "notBefore": notBefore,
		})
		return stored, submission, err
	}
	submission, err := accounts.Call("accounts.submit_batch_begin", accountId, map[string]any{
		"idempotencyKey": key, "envelopeFrom": plan.Sender, "recipients": recipients, "notBefore": notBefore,
	})
	if err != nil {
		return stored, nil, err
	}
	if submission["state"] == "building" {
		submission, err = provider.Batch(BatchUpload{
			Accounts: accounts, AccountId: accountId, Operation: "accounts.submit_batch", Submission: submission,
			Request: plan.Request, TrackingId: plan.TrackingId, Sender: plan.Sender, MessageId: messageId, Recipients: recipients,
		})
		if err != nil {
			return stored, submission, err
		}
	}
	switch submission["state"] {
	case "queued", "applying", "sent", "ambiguous":
		return stored, submission, nil
	}
	return stored, submission, errors.New("Connected-account tracking batch is not runnable")
}

// Batch appends one tracked MIME source per remaining recipient to a
// building (or editing) submission and commits it.
func (ProviderSend) Batch(upload BatchUpload) (map[string]any, error) {
	submission := upload.Submission
	submissionId := any(upload.SubmissionId)
	if upload.SubmissionId == "" {
		submissionId = submission["id"]
	}
	for index := int(RecordValues{}.NumberOf(submission["deliveryCount"])); index < len(upload.Recipients); index++ {
		recipient := upload.Recipients[index]
		request, err := ExternalSubmission{}.DeliveryRequest(upload.Request, upload.TrackingId, recipient)
		if err != nil {
			return submission, err
		}
		source := MimeMessage{}.Source(upload.Sender, request, upload.MessageId, time.Now())
		appended, err := upload.Accounts.Call(upload.Operation+"_append", upload.AccountId, map[string]any{
			"submissionId": submissionId, "revision": submission["revision"], "index": index, "recipient": recipient,
			"rawRFC822Base64": base64.StdEncoding.EncodeToString(source),
		})
		if err != nil {
			return submission, err
		}
		submission = DeliveryFields{}.Merged(appended, map[string]any{"id": RecordValues{}.Coalesce(appended["id"], submissionId)})
	}
	committed, err := upload.Accounts.Call(upload.Operation+"_commit", upload.AccountId, map[string]any{
		"submissionId": submissionId, "revision": submission["revision"],
	})
	if err != nil {
		return submission, err
	}
	return committed, nil
}

func (provider ProviderSend) accepted(plan SendPlan, accountId string, stored map[string]any, submission map[string]any) map[string]any {
	messageId := RecordValues{}.Text(stored["messageId"])
	switch {
	case plan.HasSendAt:
		return map[string]any{"scheduled": true, "id": messageId, "messageId": messageId, "threadId": stored["threadId"], "externalAccountId": accountId}
	case plan.DelayMs > 0:
		undoId := ExternalSubmission{}.UndoToken(accountId, RecordValues{}.Text(submission["id"]), messageId, "")
		return map[string]any{"sent": true, "queued": true, "messageId": messageId, "threadId": stored["threadId"], "undoId": undoId}
	}
	return map[string]any{"queued": true, "queueId": submission["id"], "messageId": messageId, "threadId": stored["threadId"], "externalAccountId": accountId}
}

func (ProviderSend) notBefore(plan SendPlan) any {
	switch {
	case plan.HasSendAt:
		return EpochTime{}.Text(plan.SendAt)
	case plan.DelayMs > 0:
		return EpochTime{}.Text(EpochTime{}.Now() + plan.DelayMs)
	}
	return nil
}

func (ProviderSend) unique(addresses []string) []string {
	seen, unique := map[string]bool{}, []string{}
	for _, address := range addresses {
		if !seen[address] {
			seen[address] = true
			unique = append(unique, address)
		}
	}
	return unique
}
