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
		Suppressed: sent.Queue.Suppressed, Senders: OwnedSenders{Users: sent.Users}, Limits: limits,
	}
}

// Schedules answers scheduled-message management.
func (stores DeliveryStores) Schedules(smtp *SmtpService) ScheduledSends {
	return ScheduledSends{Sent: stores.Sent(smtp), Reminders: stores.FollowUps()}
}

// Editing answers scheduled-message editing.
func (stores DeliveryStores) Editing(smtp *SmtpService) ScheduledEditing {
	schedules := stores.Schedules(smtp)
	return ScheduledEditing{Schedules: schedules, Senders: OwnedSenders{Users: schedules.Sent.Users}}
}

// Undo answers delayed-send undo.
func (stores DeliveryStores) Undo(smtp *SmtpService) SendUndo {
	return SendUndo{Sent: stores.Sent(smtp), Reminders: stores.FollowUps(), Archive: stores.Archive()}
}

// Drafts answers the durable draft lifecycle.
func (stores DeliveryStores) Drafts() Drafts {
	emails := stores.Emails()
	return Drafts{Emails: emails, Attachments: emails.attachmentsHelper, Users: stores.Users(), Storage: TenantStorage{Fylo: stores.Fylo}}
}

// Snoozes answers conversation snoozing.
func (stores DeliveryStores) Snoozes() Snoozes {
	emails, users := stores.Emails(), stores.Users()
	return Snoozes{
		Snoozed: &SnoozeHelper{fyloHelper: stores.Fylo}, Emails: emails, Threads: &ThreadsHelper{emailsHelper: emails}, Users: users,
		Notices: PushNotices{Users: users, Subscriptions: &PushSubscriptionsHelper{fyloHelper: stores.Fylo}, Emails: emails},
	}
}

// Routing answers inbound route and inbox-rule handling.
func (stores DeliveryStores) Routing(smtp *SmtpService) InboundRouting {
	return InboundRouting{Emails: stores.Emails(), Rules: &RulesHelper{fyloHelper: stores.Fylo}, Users: stores.Users(), Smtp: smtp}
}

// Inbound answers the inbound delivery pipeline.
func (stores DeliveryStores) Inbound(routing InboundRouting, spam *SpamFilterService) InboundDelivery {
	users := routing.Users
	return InboundDelivery{
		Emails: routing.Emails, Domains: &DomainsHelper{fyloHelper: stores.Fylo}, Users: users, Routing: routing, Spam: spam,
		FollowUps: stores.FollowUps(), Participants: MailParticipants{Contacts: &ContactsHelper{fyloHelper: stores.Fylo}, Users: users},
		Notices: PushNotices{Users: users, Subscriptions: &PushSubscriptionsHelper{fyloHelper: stores.Fylo}, Emails: routing.Emails}, Storage: TenantStorage{Fylo: stores.Fylo},
	}
}
