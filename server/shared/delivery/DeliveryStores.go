// DeliveryStores builds the stores the delivery code works on around the
// request's one FyloHelper, so every store acts as the caller the request
// authorized. Delivery reaches more stores than Tachyon lets one provider
// need (five), so its services need the FyloHelper and build the rest here.
type DeliveryStores struct {
	Fylo *FyloHelper
}

// Emails answers the email store (with its attachment and source stores).
func (stores DeliveryStores) Emails() *EmailsHelper {
	return &EmailsHelper{fyloHelper: stores.Fylo, attachmentsHelper: &AttachmentsHelper{fyloHelper: stores.Fylo}, rawMessagesHelper: &RawMessagesHelper{fyloHelper: stores.Fylo}}
}

// Users answers the user store.
func (stores DeliveryStores) Users() *UsersHelper {
	return &UsersHelper{fyloHelper: stores.Fylo}
}

// Queue answers the outbound queue.
func (stores DeliveryStores) Queue(smtp *SmtpService) OutboundQueue {
	emails := stores.Emails()
	return OutboundQueue{
		Outbox: &OutboxHelper{fyloHelper: stores.Fylo}, Emails: emails, Scheduled: &ScheduledHelper{fyloHelper: stores.Fylo},
		Attachments: emails.attachmentsHelper, Suppressed: &SuppressedHelper{fyloHelper: stores.Fylo}, Users: stores.Users(), Smtp: smtp,
	}
}

// Sent answers outbound mail acceptance.
func (stores DeliveryStores) Sent(smtp *SmtpService) SentMail {
	queue := stores.Queue(smtp)
	participants := MailParticipants{Contacts: &ContactsHelper{fyloHelper: stores.Fylo}, Users: queue.Users}
	return SentMail{
		Emails: queue.Emails, Attachments: queue.Attachments, RawMessages: queue.Emails.rawMessagesHelper,
		Scheduled: queue.Scheduled, Outbox: queue.Outbox, Queue: queue, Participants: participants, Users: queue.Users,
	}
}

// FollowUps answers the follow-up reminders.
func (stores DeliveryStores) FollowUps() FollowUpReminders {
	return FollowUpReminders{Reminders: &FollowUpRemindersHelper{fyloHelper: stores.Fylo}, Emails: stores.Emails(), Users: stores.Users()}
}

// Archive answers Send & Archive's conversation archive.
func (stores DeliveryStores) Archive() ConversationArchive {
	emails := stores.Emails()
	return ConversationArchive{Emails: emails, Threads: &ThreadsHelper{emailsHelper: emails}, Users: stores.Users()}
}
