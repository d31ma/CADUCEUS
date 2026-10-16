import "slices"

// @Service
type RemindersService struct {
	// @Needs
	remindersClient           *RemindersClient
	applicationEventPublisher ApplicationEventPublisher
	// @SessionState
	Items         []ReminderRow
	Loaded        bool
	signedIn      bool
	authorization string
}

// @OnEvent({"replay":true})
func (reminders *RemindersService) SessionChanged(session SessionChanged) {
	reminders.signedIn = session.SignedIn
	reminders.authorization = "Bearer " + session.Token
	reminders.loadWhenSignedIn()
	reminders.applicationEventPublisher.PublishEvent(ViewSettled{Source: "reminders"}, false)
}

// Load reads the owner's follow-up reminders. A failed read shows none.
func (reminders *RemindersService) Load() {
	reminders.Loaded = false
	found, _ := reminders.remindersClient.List(reminders.authorization)
	reminders.Items = reminders.rows(found)
	reminders.Loaded = true
}

// Open opens the message that a reminder belongs to.
func (reminders *RemindersService) Open(emailID string) {
	reminders.applicationEventPublisher.PublishEvent(ComposeUtils{}.Message(emailID), false)
}

// Settled renders the page after a listener that waited for the server.
//
// @OnEvent
func (reminders *RemindersService) Settled(_ ViewSettled) {}

// Dismiss deletes one reminder and says whether that worked.
func (reminders *RemindersService) Dismiss(id string) {
	_, failure := reminders.remindersClient.Dismiss(reminders.authorization, map[string]any{"id": id})
	reminders.Items = reminders.withoutUnless(id, failure)
	reminders.applicationEventPublisher.PublishEvent(ToastRequested{Message: reminders.dismissMessage(failure)}, false)
}

func (reminders *RemindersService) loadWhenSignedIn() {
	if reminders.signedIn {
		reminders.Load()
	}
}

func (reminders *RemindersService) rows(found []Reminder) []ReminderRow {
	rows := []ReminderRow{}
	for _, reminder := range found {
		rows = append(rows, ReminderRow{
			ID: reminder.ID, EmailID: reminder.EmailID, NoticeEmailID: reminder.NoticeEmailID,
			Subject: reminder.Subject, RecipientLabel: reminder.Recipient, Status: reminder.Status,
			RemindAtLabel: DateLabelUtils{}.Numeric(reminder.RemindAt),
		})
	}
	return rows
}

func (reminders *RemindersService) withoutUnless(id string, failure error) []ReminderRow {
	if failure != nil {
		return reminders.Items
	}
	return slices.DeleteFunc(slices.Clone(reminders.Items), func(row ReminderRow) bool { return row.ID == id })
}

func (reminders *RemindersService) dismissMessage(failure error) string {
	if failure != nil {
		return "Could not dismiss reminder."
	}
	return "Reminder dismissed."
}
