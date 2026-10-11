// FollowUpReminderService schedules, lists and surfaces the "remind me if
// nobody replies" reminders of sent mail.
// @Service
type FollowUpReminderService struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Reminders answers the request's follow-up reminders.
func (service *FollowUpReminderService) Reminders() FollowUpReminders {
	return DeliveryStores{Fylo: service.fyloHelper}.FollowUps()
}
