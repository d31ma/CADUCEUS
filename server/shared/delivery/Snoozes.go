import (
	"sort"
	"strings"
)

// Snoozes snoozes whole conversations: every inbox message of the selected
// RFC thread moves to Snoozed until its time, then returns unread (to the
// top of the inbox unless the owner turned that off) with a notice. Sent
// and already-moved messages stay where the user put them.
type Snoozes struct {
	Snoozed *SnoozeHelper
	Emails  *EmailsHelper
	Threads *ThreadsHelper
	Users   *UsersHelper
	Notices PushNotices
}

// SnoozeThread answers {snoozed, snoozeId, threadId, count, until}; a
// repeated clientRequestId replays the first answer. input holds emailId,
// until (milliseconds), label, clientRequestId, ownerEmail and domains.
func (snoozes Snoozes) SnoozeThread(input map[string]any, domains []string) (map[string]any, error) {
	values := RecordValues{}
	now := EpochTime{}.Now()
	ownerInput, emailId, clientRequestId := values.Text(input["ownerEmail"]), values.Text(input["emailId"]), values.Text(input["clientRequestId"])
	until, _ := DeliveryFields{}.Millis(input["until"])
	if clientRequestId != "" {
		_, replay, err := snoozes.Snoozed.FindByClientRequestId(ownerInput, clientRequestId)
		if err != nil {
			return nil, err
		}
		if replay != nil {
			return snoozes.replayed(replay), nil
		}
	}
	if until <= now {
		return nil, &DeliveryRefusal{Message: "until must be in the future", Status: 400}
	}
	_, selected, err := snoozes.Emails.FindById(emailId)
	if err != nil {
		return nil, err
	}
	if selected == nil || !(TextValue{}).Includes(domains, selected["domain"]) || !(EmailsRecords{}).CanAccessEmail(selected, ownerInput) {
		return nil, &DeliveryRefusal{Message: "Email not found", Status: 404}
	}
	thread, err := snoozes.Threads.FindForEmail(domains, emailId, ownerInput)
	if err != nil {
		return nil, err
	}
	if thread == nil {
		return nil, &DeliveryRefusal{Message: "Conversation not found", Status: 404}
	}
	eligible := []string{}
	roles := CategoryRoles{Users: snoozes.Users}
	for _, email := range thread.Emails {
		folder := values.Text(values.Or(email["folder"], "inbox"))
		if (TextValue{}).Includes(domains, email["domain"]) && folder == roles.PathFor(ownerInput, values.Text(email["domain"]), "inbox") {
			eligible = append(eligible, values.Text(email["id"]))
		}
	}
	if len(eligible) == 0 {
		return nil, &DeliveryRefusal{Message: "Only inbox conversations can be snoozed", Status: 409}
	}
	return snoozes.store(input, selected, thread.Id, eligible, now, until)
}

func (snoozes Snoozes) store(input map[string]any, selected map[string]any, threadId string, eligible []string, now int64, until int64) (map[string]any, error) {
	values := RecordValues{}
	ownerEmail := strings.ToLower(values.Text(input["ownerEmail"]))
	existingDocumentId, existing, err := snoozes.Snoozed.FindActiveForThread(ownerEmail, threadId)
	if err != nil {
		return nil, err
	}
	id := values.Text(existing["id"])
	if id == "" {
		id = DeliveryFields{}.Serial("snooze-")
	}
	emailIds := DeliveryFields{}.Strings(existing["emailIds"])
	if len(emailIds) == 0 && values.Text(existing["emailId"]) != "" {
		emailIds = []string{values.Text(existing["emailId"])}
	}
	for _, emailId := range eligible {
		if !(TextValue{}).Includes(emailIds, emailId) {
			emailIds = append(emailIds, emailId)
		}
	}
	label := values.Text(input["label"])
	if label == "" {
		label = "Custom"
	}
	record := map[string]any{
		"id": id, "emailId": input["emailId"], "threadId": threadId, "ownerEmail": ownerEmail, "domain": selected["domain"],
		"emailIds": DeliveryFields{}.Items(emailIds), "until": until, "snoozedAt": now, "label": TextValue{}.Prefix(label, 60),
		"status": "active", "lastError": "",
	}
	if clientRequestId := values.Text(input["clientRequestId"]); clientRequestId != "" {
		record["clientRequestId"] = clientRequestId
	}
	if existingDocumentId != "" {
		err = snoozes.Snoozed.Update(existingDocumentId, record)
	} else {
		existingDocumentId, err = snoozes.Snoozed.Put(record)
	}
	if err != nil {
		return nil, err
	}
	_, err = snoozes.Emails.UpdateByIds(eligible, map[string]any{
		"folder": "snoozed", "snoozeId": id, "snoozeThreadId": threadId, "snoozedUntil": EpochTime{}.Text(until), "snoozeLabel": record["label"],
	})
	if err != nil {
		_ = snoozes.Snoozed.Update(existingDocumentId, map[string]any{"status": "canceled", "lastError": "message move failed"})
		return nil, err
	}
	return map[string]any{"snoozed": true, "snoozeId": id, "threadId": threadId, "count": len(eligible), "until": EpochTime{}.Text(until)}, nil
}

func (Snoozes) replayed(replay map[string]any) map[string]any {
	count := len(DeliveryFields{}.Strings(replay["emailIds"]))
	if count == 0 {
		count = 1
	}
	until, _ := DeliveryFields{}.Millis(replay["until"])
	return map[string]any{"snoozed": replay["status"] == "active", "snoozeId": replay["id"], "threadId": replay["threadId"], "count": count, "until": EpochTime{}.Text(until)}
}

// ListActive answers the owner's active snoozes, soonest first.
func (snoozes Snoozes) ListActive(ownerEmail string, domains []string) ([]map[string]any, error) {
	records, err := snoozes.Snoozed.ListActive(ownerEmail)
	if err != nil {
		return nil, err
	}
	values := RecordValues{}
	results := []map[string]any{}
	for _, stored := range records {
		record := stored.Data
		_, selected, err := snoozes.Emails.FindById(values.Text(record["emailId"]))
		if err != nil {
			return nil, err
		}
		if selected == nil || !(TextValue{}).Includes(domains, selected["domain"]) || !(EmailsRecords{}).CanAccessEmail(selected, ownerEmail) {
			continue
		}
		count := len(DeliveryFields{}.Strings(record["emailIds"]))
		if count == 0 {
			count = 1
		}
		until, _ := DeliveryFields{}.Millis(record["until"])
		results = append(results, map[string]any{
			"id": record["id"], "threadId": values.Or(record["threadId"], values.Or(selected["threadId"], selected["id"])),
			"emailId": record["emailId"], "subject": selected["subject"], "sender": selected["sender"], "domain": selected["domain"],
			"count": count, "label": values.Or(record["label"], "Custom"), "snoozedUntil": EpochTime{}.Text(until),
		})
	}
	sort.SliceStable(results, func(left, right int) bool {
		return (TextOrder{}).Compare(values.Text(results[left]["snoozedUntil"]), values.Text(results[right]["snoozedUntil"])) < 0
	})
	return results, nil
}

// Undo returns the owner's active snooze without marking it unread.
func (snoozes Snoozes) Undo(id string, ownerEmail string) (map[string]any, error) {
	documentId, record, err := snoozes.Snoozed.FindById(id)
	if err != nil {
		return nil, err
	}
	missing := &DeliveryRefusal{Message: "Snooze not found", Status: 404}
	if documentId == "" || record == nil || record["status"] == "completed" || record["status"] == "canceled" {
		return nil, missing
	}
	if owner := (RecordValues{}).Text(record["ownerEmail"]); owner != "" && !strings.EqualFold(owner, ownerEmail) {
		return nil, missing
	}
	moved, err := snoozes.release(documentId, record, map[string]any{"now": EpochTime{}.Now(), "status": "canceled"})
	if err != nil {
		return nil, err
	}
	return map[string]any{"snoozed": false, "snoozeId": id, "count": moved}, nil
}

// ProcessDue returns every due snooze, unread, and answers {due, returned,
// failed, notified}.
func (snoozes Snoozes) ProcessDue(now int64) (map[string]any, error) {
	due, err := snoozes.Snoozed.ListDue(now, 100)
	if err != nil {
		return nil, err
	}
	returned, failed, notified := 0, 0, 0
	for _, stored := range due {
		count, err := snoozes.returned(stored, now)
		if err != nil {
			failed++
			_ = snoozes.Snoozed.Update(stored.Id, map[string]any{"lastError": TextValue{}.Prefix(err.Error(), 200)})
			continue
		}
		returned += count
		if count > 0 {
			notified++
		}
	}
	return map[string]any{"due": len(due), "returned": returned, "failed": failed, "notified": notified}, nil
}

func (snoozes Snoozes) returned(stored FyloRecord, now int64) (int, error) {
	record := stored.Data
	toTop := true
	if owner := (RecordValues{}).Text(record["ownerEmail"]); owner != "" {
		_, user, err := snoozes.Users.FindByEmail(owner)
		if err != nil {
			return 0, err
		}
		preferences, _ := RecordValues{}.Object(user["mailPreferences"])
		toTop = preferences["unsnoozeToTop"] != false
	}
	return snoozes.release(stored.Id, record, map[string]any{"now": now, "status": "completed", "markUnread": true, "notify": true, "returnToTop": toTop})
}

// release returns only the messages still in Snoozed, so a conversation the
// user already moved elsewhere stays put, and answers how many it moved.
func (snoozes Snoozes) release(documentId string, record map[string]any, options map[string]any) (int, error) {
	values := RecordValues{}
	ids := DeliveryFields{}.Strings(record["emailIds"])
	if len(ids) == 0 {
		ids = []string{values.Text(record["emailId"])}
	}
	owner := values.Text(record["ownerEmail"])
	messages := []map[string]any{}
	for _, id := range ids {
		_, email, err := snoozes.Emails.FindById(id)
		if err != nil {
			return 0, err
		}
		if email != nil && email["folder"] == "snoozed" && (owner == "" || (EmailsRecords{}).CanAccessEmail(email, owner)) {
			messages = append(messages, email)
		}
	}
	now, _ := DeliveryFields{}.Millis(options["now"])
	for _, email := range messages {
		if err := snoozes.returnOne(email, owner, options, now); err != nil {
			return 0, err
		}
	}
	if err := snoozes.Snoozed.Update(documentId, map[string]any{"status": options["status"], "completedAt": now, "lastError": ""}); err != nil {
		return 0, err
	}
	if options["notify"] == true && len(messages) > 0 {
		sort.SliceStable(messages, func(left, right int) bool {
			return (TextOrder{}).Compare(values.Text(messages[right]["receivedAt"]), values.Text(messages[left]["receivedAt"])) < 0
		})
		snoozes.Notices.Unsnoozed(messages[0])
	}
	return len(messages), nil
}

func (snoozes Snoozes) returnOne(email map[string]any, owner string, options map[string]any, now int64) error {
	values := RecordValues{}
	emailDocumentId, _, err := snoozes.Emails.FindById(values.Text(email["id"]))
	if err != nil || emailDocumentId == "" {
		return err
	}
	recipient := values.Text(values.Or(owner, values.Or(email["ownerEmail"], email["recipient"])))
	patch := map[string]any{"folder": CategoryRoles{Users: snoozes.Users}.PathFor(recipient, values.Text(email["domain"]), "inbox")}
	if options["markUnread"] == true {
		patch["read"] = false
		if options["returnToTop"] == true {
			patch["receivedAt"] = EpochTime{}.Text(now)
		}
	}
	return snoozes.Emails.Update(emailDocumentId, patch)
}
