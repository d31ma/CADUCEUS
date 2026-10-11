// CalendarImportService applies the iCalendar invitations of delivered
// mail to the recipients' calendars.
// @Service
type CalendarImportService struct {
	// @Needs
	calendarEventsHelper *CalendarEventsHelper
}

// importFromInboundEmail applies every invitation in one delivered message.
// Inbound delivery calls it; it stays lowercase until that route is merged
// in, because nothing in the calendar slice does.
func (service *CalendarImportService) importFromInboundEmail(context ImportContext) ImportResult {
	return CalendarImport{Events: service.calendarEventsHelper}.FromMessage(context)
}
