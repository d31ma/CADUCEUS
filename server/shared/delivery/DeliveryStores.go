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

// Acceptance answers POST /send's acceptance with the request's journal and
// rate limits.
func (stores DeliveryStores) Acceptance(smtp *SmtpService, journal SendIdempotency, limits *RateLimitService) SendAcceptance {
	sent := stores.Sent(smtp)
	return SendAcceptance{
		Sent: sent, Journal: journal, Reminders: stores.FollowUps(), Archive: stores.Archive(),
		Suppressed: sent.Queue.Suppressed, Senders: SenderAddresses{Users: sent.Users}, Limits: limits,
	}
}

// Schedules answers scheduled-message management.
func (stores DeliveryStores) Schedules(smtp *SmtpService) ScheduledSends {
	return ScheduledSends{Sent: stores.Sent(smtp), Reminders: stores.FollowUps()}
}

// Editing answers scheduled-message editing.
func (stores DeliveryStores) Editing(smtp *SmtpService) ScheduledEditing {
	schedules := stores.Schedules(smtp)
	return ScheduledEditing{Schedules: schedules, Senders: SenderAddresses{Users: schedules.Sent.Users}}
}

// Undo answers delayed-send undo.
func (stores DeliveryStores) Undo(smtp *SmtpService) SendUndo {
	return SendUndo{Sent: stores.Sent(smtp), Reminders: stores.FollowUps(), Archive: stores.Archive()}
}
