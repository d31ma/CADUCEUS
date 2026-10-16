// CalendarClient is the /calendar route group. The shell counts the events of
// the next two weeks for the sidebar badge.
// @HTTPClient("/calendar")
type CalendarClient interface {
	// @GetRequest("/events")
	Events(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("start")
		start string,
		// @RequestParam("end")
		end string,
	) ([]CalendarEventSummary, error)
}
