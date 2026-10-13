// CalendarImportService applies the iCalendar invitations of delivered
// mail to the recipients' calendars.
// @Service
type CalendarImportService struct {
	// @Needs
	calendarEventsHelper *CalendarEventsHelper
}

// ImportFromInboundEmail applies every invitation in one delivered message;
// failures only log and never block delivery.
func (service *CalendarImportService) ImportFromInboundEmail(context ImportContext) ImportResult {
	return CalendarImport{Events: service.calendarEventsHelper}.FromMessage(context)
}
