import (
	"log"
	"regexp"
	"sort"
	"strings"
)

// FollowUpReminders keeps "remind me if nobody replies" reminders: it
// schedules them, surfaces due ones as unread inbox notices, and completes
// them when a participant replies. Due passes hold a process lock, as the
// JavaScript server serialized them in its one process.
type FollowUpReminders struct {
	Reminders *FollowUpRemindersHelper
	Emails    *EmailsHelper
	Users     *UsersHelper
}

// Schedule stores (or moves) the owner's reminder for one message and
// answers it. The input holds ownerEmail, emailId, recipient, subject,
// threadId and remindAt (milliseconds).
func (reminders FollowUpReminders) Schedule(input map[string]any) (map[string]any, error) {
	text := RecordValues{}.Text
	ownerEmail := strings.ToLower(Security{}.Trim(text(input["ownerEmail"])))
	existing, err := reminders.Reminders.FindActiveForEmail(ownerEmail, text(input["emailId"]))
	if err != nil {
		return nil, err
	}
	threadId := text(input["threadId"])
	if existing != nil {
		patch := map[string]any{
			"remindAt": input["remindAt"], "recipient": input["recipient"], "subject": input["subject"],
			"threadKey": reminders.ThreadKey(text(input["subject"])), "status": "scheduled",
		}
		if threadId != "" {
			patch["threadId"] = threadId
		}
		if err := reminders.Reminders.Update(text(existing["docId"]), patch); err != nil {
			return nil, err
		}
		return DeliveryFields{}.Merged(existing, map[string]any{"remindAt": input["remindAt"], "status": "scheduled"}), nil
	}
	reminder := map[string]any{
		"id":         DeliveryFields{}.Serial("reminder-"),
		"ownerEmail": ownerEmail,
		"domain":     DeliveryFields{}.Domain(ownerEmail),
		"emailId":    input["emailId"],
		"recipient":  strings.ToLower(text(input["recipient"])),
		"subject":    input["subject"],
		"threadKey":  reminders.ThreadKey(text(input["subject"])),
		"remindAt":   input["remindAt"],
		"createdAt":  EpochTime{}.Now(),
		"status":     "scheduled",
	}
	if threadId != "" {
		reminder["threadId"] = threadId
	}
	_, err = reminders.Reminders.Put(reminder)
	return reminder, err
}

// ListForOwner surfaces the owner's due reminders, then answers the ones
// still open, soonest first.
func (reminders FollowUpReminders) ListForOwner(ownerEmail string) ([]map[string]any, error) {
	if _, err := reminders.ProcessDue(ownerEmail, (EpochTime{}).Now()); err != nil {
		return nil, err
	}
	listed, err := reminders.Reminders.ListForOwner(strings.ToLower(ownerEmail))
	if err != nil {
		return nil, err
	}
	open := []map[string]any{}
	for _, reminder := range listed {
		if reminder["status"] != "dismissed" && reminder["status"] != "completed" {
			open = append(open, reminder)
		}
	}
	sort.SliceStable(open, func(left, right int) bool {
		return RecordValues{}.Numeric(open[left]["remindAt"]) < RecordValues{}.Numeric(open[right]["remindAt"])
	})
	return open, nil
}

// ProcessDue surfaces one mailbox's due reminders and answers how many it
// processed.
func (reminders FollowUpReminders) ProcessDue(ownerEmail string, now int64) (int, error) {
	processed := 0
	err := ProcessLock{Name: "reminders"}.Held(func() error {
		pending, err := reminders.Reminders.ListPending(strings.ToLower(ownerEmail))
		if err != nil {
			return err
		}
		processed = reminders.processPending(pending, now)["processed"]
		return nil
	})
	return processed, err
}

// ProcessDueAll surfaces the due reminders of every mailbox and answers
// {due, surfaced, reconciled, failed}.
func (reminders FollowUpReminders) ProcessDueAll(now int64) (map[string]any, error) {
	result := map[string]any{}
	err := ProcessLock{Name: "reminders"}.Held(func() error {
		pending, err := reminders.Reminders.ListPendingAll()
		if err != nil {
			return err
		}
		counts := reminders.processPending(pending, now)
		result = map[string]any{"due": counts["due"], "surfaced": counts["surfaced"], "reconciled": counts["reconciled"], "failed": counts["failed"]}
		return nil
	})
	return result, err
}

// processPending materializes due reminders as unread inbox notices. The
// notice id is deterministic, so a restart never surfaces one twice.
func (reminders FollowUpReminders) processPending(pending []map[string]any, now int64) map[string]int {
	result := map[string]int{"due": 0, "processed": 0, "surfaced": 0, "reconciled": 0, "failed": 0}
	for _, reminder := range pending {
		if (RecordValues{}).Numeric(reminder["remindAt"]) > float64(now) {
			continue
		}
		result["due"]++
		surfaced, err := reminders.surface(reminder, now)
		if err != nil {
			result["failed"]++
			log.Printf("[reminders] failed to surface due reminder: %v", err)
			continue
		}
		if surfaced {
			result["surfaced"]++
		} else {
			result["reconciled"]++
		}
		result["processed"]++
	}
	return result
}

func (reminders FollowUpReminders) surface(reminder map[string]any, now int64) (bool, error) {
	text := RecordValues{}.Text
	noticeEmailId := "follow-up-" + text(reminder["id"])
	existing, _, err := reminders.Emails.FindById(noticeEmailId)
	if err != nil {
		return false, err
	}
	if existing == "" {
		owner, domain := text(reminder["ownerEmail"]), text(reminder["domain"])
		notice := map[string]any{
			"id": noticeEmailId, "domain": domain, "ownerEmail": owner, "recipient": owner, "sender": owner,
			"subject":    "Re: " + text(reminder["subject"]),
			"body":       "Follow-up reminder: nobody has replied to your message to " + text(reminder["recipient"]) + ".",
			"folder":     CategoryMappings{}.OfOwner(reminders.Users, owner).PathFor(domain, "inbox"),
			"read":       false,
			"starred":    false,
			"receivedAt": EpochTime{}.Text(now),
			"processed":  true,
		}
		if threadId := text(reminder["threadId"]); threadId != "" {
			notice["threadId"] = threadId
		}
		if _, err := reminders.Emails.Put(notice); err != nil {
			return false, err
		}
	}
	return existing == "", reminders.Reminders.Update(text(reminder["docId"]), map[string]any{"status": "due", "dueAt": now, "noticeEmailId": noticeEmailId})
}

// ResolveForReply completes the owner's scheduled reminders that a reply
// from one of their recipients answers, and answers how many.
func (reminders FollowUpReminders) ResolveForReply(ownerEmail string, sender string, subject string, threadId string) (int, error) {
	pending, err := reminders.Reminders.ListPending(strings.ToLower(ownerEmail))
	if err != nil {
		return 0, err
	}
	key, from := reminders.ThreadKey(subject), strings.ToLower(sender)
	completed := 0
	for _, reminder := range pending {
		storedThread := RecordValues{}.Text(reminder["threadId"])
		sameThread := reminder["threadKey"] == key
		if storedThread != "" && threadId != "" {
			sameThread = storedThread == threadId
		}
		if !sameThread || !reminders.recipients(RecordValues{}.Text(reminder["recipient"]))[from] {
			continue
		}
		if err := reminders.Reminders.Update(RecordValues{}.Text(reminder["docId"]), map[string]any{"status": "completed", "completedAt": (EpochTime{}).Now()}); err != nil {
			return completed, err
		}
		completed++
	}
	return completed, nil
}

// Dismiss dismisses the owner's reminder with the id, and reports whether
// there was one.
func (reminders FollowUpReminders) Dismiss(id string, ownerEmail string) (bool, error) {
	documentId, reminder, err := reminders.Reminders.FindById(id)
	if err != nil || documentId == "" || reminder == nil || reminder["ownerEmail"] != strings.ToLower(ownerEmail) {
		return false, err
	}
	return true, reminders.Reminders.Update(documentId, map[string]any{"status": "dismissed", "completedAt": EpochTime{}.Now()})
}

// DismissForEmail dismisses the owner's open reminder for one message, and
// reports whether there was one.
func (reminders FollowUpReminders) DismissForEmail(emailId string, ownerEmail string) (bool, error) {
	reminder, err := reminders.Reminders.FindActiveForEmail(strings.ToLower(ownerEmail), emailId)
	if err != nil || reminder == nil {
		return false, err
	}
	return true, reminders.Reminders.Update(RecordValues{}.Text(reminder["docId"]), map[string]any{"status": "dismissed", "completedAt": EpochTime{}.Now()})
}

// ThreadKey answers the subject without its reply and forward prefixes.
func (FollowUpReminders) ThreadKey(subject string) string {
	stripped := regexp.MustCompile(`(?i)^(?:(?:re|fw|fwd):\s*)+`).ReplaceAllString(subject, "")
	return strings.ToLower(Security{}.Trim(stripped))
}

func (FollowUpReminders) recipients(value string) map[string]bool {
	set := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		if address := strings.ToLower(Security{}.Trim(item)); address != "" {
			set[address] = true
		}
	}
	return set
}
