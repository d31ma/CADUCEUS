// SendUndo cancels a delayed send inside its undo window: a hosted outbox
// entry (restoring a Send & Archive source) or a connected-account
// submission named by an "extundo." token. Only the sender's own mailbox may
// undo it.
type SendUndo struct {
	Sent      SentMail
	Reminders FollowUpReminders
	Archive   *ConversationArchiveService
}

// Undo answers the route answer of one undo.
func (undo SendUndo) Undo(caller RouteCaller, undoId string) (any, error) {
	missing := map[string]any{"error": "Undo ID not found or already processed", "notFound": true}
	if external := (ExternalSubmission{}).ParsedUndoToken(undoId); external != nil {
		return undo.external(caller, external, missing)
	}
	documentId, entry, err := undo.Sent.Outbox.FindById(undoId)
	if err != nil || documentId == "" || entry == nil {
		return missing, err
	}
	values := RecordValues{}
	if values.Coalesce(entry["owner"], entry["sender"]) != caller.Email {
		return missing, nil
	}
	state := values.Text(entry["archiveStateJson"])
	restored := 0
	if state != "" {
		if restored, err = undo.Archive.Restore(state, caller.Email, caller.Domains); err != nil {
			return nil, err
		}
	}
	if err := undo.Sent.Outbox.Delete(documentId); err != nil {
		return nil, err
	}
	if _, err := undo.Reminders.DismissForEmail(undoId, caller.Email); err != nil {
		return nil, err
	}
	emailDocumentId, _, err := undo.Sent.Emails.FindById(undoId)
	if err == nil && emailDocumentId != "" {
		err = undo.Sent.Emails.Delete(emailDocumentId, undoId)
	}
	if err != nil {
		return nil, err
	}
	result := map[string]any{"undone": true}
	if state != "" {
		result["archiveRestored"] = restored
	}
	return result, nil
}

func (undo SendUndo) external(caller RouteCaller, external map[string]any, missing map[string]any) (any, error) {
	values := RecordValues{}
	messageId := values.Text(external["messageId"])
	_, email, err := undo.Sent.Emails.FindById(messageId)
	if err != nil {
		return nil, err
	}
	if email == nil || email["ownerEmail"] != caller.Email {
		return missing, nil
	}
	accounts := ExternalAccounts{}.For(caller.Token)
	if _, err := accounts.OnAccount("accounts.cancel_submission", values.Text(external["accountId"]), map[string]any{"submissionId": external["submissionId"]}); err != nil {
		return accounts.Refusal(err), nil
	}
	if _, err := undo.Reminders.DismissForEmail(messageId, caller.Email); err != nil {
		return nil, err
	}
	if err := undo.Sent.RollbackExternalAccepted(messageId); err != nil {
		return nil, err
	}
	if external["archiveActionId"] == nil {
		return map[string]any{"undone": true}, nil
	}
	// Restoring a connected-account source runs through the journaled mail
	// actions, which mail reading owns.
	return map[string]any{"undone": true, "archiveRestored": 0, "archiveError": "Connected-account source restore is not available"}, nil
}
