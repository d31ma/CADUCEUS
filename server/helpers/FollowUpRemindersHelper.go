// FollowUpRemindersHelper keeps the reminders to follow up on sent mail.
// @Helper
type FollowUpRemindersHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores one reminder and answers its document id.
func (helper *FollowUpRemindersHelper) Put(reminder map[string]any) (string, error) {
	return helper.store().Put(reminder)
}

// ListForOwner answers the reminders of one owner as {...reminder, docId},
// in FYLO's order.
func (helper *FollowUpRemindersHelper) ListForOwner(ownerEmail string) ([]map[string]any, error) {
	documents, err := helper.store().Where("ownerEmail", ownerEmail)
	return helper.withDocumentIds(documents), err
}

// ListPending answers the scheduled reminders of one owner.
func (helper *FollowUpRemindersHelper) ListPending(ownerEmail string) ([]map[string]any, error) {
	reminders, err := helper.ListForOwner(ownerEmail)
	pending := []map[string]any{}
	for _, reminder := range reminders {
		if reminder[helper.statusField()] == helper.scheduled() {
			pending = append(pending, reminder)
		}
	}
	return pending, err
}

// ListPendingAll answers every scheduled reminder, for the system worker that
// sends due reminders.
func (helper *FollowUpRemindersHelper) ListPendingAll() ([]map[string]any, error) {
	documents, err := helper.store().Where(helper.statusField(), helper.scheduled())
	return helper.withDocumentIds(documents), err
}

// FindById answers the document id and reminder with the id, or "" and nil.
func (helper *FollowUpRemindersHelper) FindById(id string) (string, map[string]any, error) {
	return helper.store().FindById(id)
}

// FindActiveForEmail answers the owner's scheduled or due reminder for one
// message, or nil.
func (helper *FollowUpRemindersHelper) FindActiveForEmail(ownerEmail string, emailId string) (map[string]any, error) {
	reminders, err := helper.ListForOwner(ownerEmail)
	for _, reminder := range reminders {
		status := reminder[helper.statusField()]
		if reminder["emailId"] == emailId && (status == helper.scheduled() || status == "due") {
			return reminder, nil
		}
	}
	return nil, err
}

// Update merges a patch into one reminder.
func (helper *FollowUpRemindersHelper) Update(documentId string, patch map[string]any) error {
	return helper.store().Patch(documentId, patch)
}

// withDocumentIds answers {...reminder, docId}: the document id always wins.
func (FollowUpRemindersHelper) withDocumentIds(documents []FyloRecord) []map[string]any {
	reminders := []map[string]any{}
	for _, document := range documents {
		reminder := document.Data
		reminder["docId"] = document.Id
		reminders = append(reminders, reminder)
	}
	return reminders
}

func (FollowUpRemindersHelper) statusField() string { return "status" }

func (FollowUpRemindersHelper) scheduled() string { return "scheduled" }

func (helper *FollowUpRemindersHelper) store() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().FollowUpReminders}
}
