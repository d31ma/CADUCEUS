import (
	"fmt"
	"os"
	"strings"
)

// DueFollowUps resurfaces due follow-up reminders of one mailbox as unread
// local inbox notices. It is restart-safe: the notice id is derived from the
// reminder, so a reminder never surfaces twice.
type DueFollowUps struct {
	Reminders  *FollowUpRemindersHelper
	Emails     *EmailsHelper
	Categories *CategoryMappingService
}

// Process surfaces the owner's reminders due at now and answers how many it
// processed. A reminder that fails is logged and left for the next pass.
func (due DueFollowUps) Process(ownerEmail string, now int64) (int, error) {
	pending, err := due.Reminders.ListPending(strings.ToLower(ownerEmail))
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, reminder := range pending {
		if (RecordValues{}).NumberOf(reminder["remindAt"]) > float64(now) {
			continue
		}
		if err := due.surface(reminder, now); err != nil {
			fmt.Fprintln(os.Stderr, "[reminders] failed to surface due reminder:", err.Error())
			continue
		}
		processed++
	}
	return processed, nil
}

func (due DueFollowUps) surface(reminder map[string]any, now int64) error {
	text := TextValue{}
	noticeId := "follow-up-" + text.Of(reminder["id"])
	existing, _, err := due.Emails.FindById(noticeId)
	if err != nil {
		return err
	}
	if existing == "" {
		if err := due.notice(reminder, noticeId, now); err != nil {
			return err
		}
	}
	return due.Reminders.Update(text.Of(reminder["docId"]), map[string]any{"status": "due", "dueAt": now, "noticeEmailId": noticeId})
}

func (due DueFollowUps) notice(reminder map[string]any, noticeId string, now int64) error {
	text := TextValue{}
	owner := text.Of(reminder["ownerEmail"])
	mappings, err := due.Categories.ForOwner(owner)
	if err != nil {
		return err
	}
	folder := mappings.PathFor(text.Of(reminder["domain"]), "inbox")
	if folder == "" {
		folder = "inbox"
	}
	notice := map[string]any{
		"id": noticeId, "domain": reminder["domain"], "ownerEmail": owner, "recipient": owner, "sender": owner,
		"subject": "Re: " + text.Of(reminder["subject"]),
		"body":    "Follow-up reminder: nobody has replied to your message to " + text.Of(reminder["recipient"]) + ".",
		"folder":  folder, "read": false, "starred": false, "receivedAt": EpochTime{}.Text(now), "processed": true,
	}
	if text.Truthy(reminder["threadId"]) {
		notice["threadId"] = reminder["threadId"]
	}
	_, err = due.Emails.Put(notice)
	return err
}
