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

	// RespondToInvite answers a meeting invitation and mirrors the answer to
	// the organizer.
	//
	// @PostRequest("/invites/respond")
	RespondToInvite(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		response BehaviorValue,
	) (BehaviorValue, error)
}
