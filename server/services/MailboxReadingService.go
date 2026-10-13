// MailboxReadingService gathers the stores a message's full view reads
// besides the email itself: attachments, retained sources, calendar events
// and domain migrations.
// @Service
type MailboxReadingService struct {
	// @Needs
	attachmentsHelper *AttachmentsHelper
	// @Needs
	calendarEventsHelper *CalendarEventsHelper
	// @Needs
	domainMigrationsHelper *DomainMigrationsHelper
	// @Needs
	rawMessagesHelper *RawMessagesHelper
}

// Completed answers parts with the reading stores filled in.
func (service *MailboxReadingService) Completed(parts MailboxParts) MailboxParts {
	parts.Attachments = service.attachmentsHelper
	parts.Calendar = service.calendarEventsHelper
	parts.Migrations = service.domainMigrationsHelper
	parts.RawMessages = service.rawMessagesHelper
	return parts
}
