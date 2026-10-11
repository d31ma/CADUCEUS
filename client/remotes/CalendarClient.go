// CalendarClient is the /calendar route group. The shell counts the events of
// the next two weeks for the sidebar badge. The calendar page reads and writes
// the events themselves; its writes answer BehaviorValue, so their arguments
// are BehaviorValue too.
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
	) ([]CalendarEvent, error)

	// @GetRequest("/team-events")
	TeamEvents(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("start")
		start string,
		// @RequestParam("end")
		end string,
	) ([]CalendarEvent, error)

	// @GetRequest("/calendars")
	Calendars(
		// @RequestHeader("Authorization")
		authorization string,
	) (ProviderCalendarList, error)

	// @GetRequest("/events/{id}")
	Event(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (CalendarEvent, error)

	// @PostRequest("/events")
	Create(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		event BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/events/{id}")
	Update(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestParam("scope")
		scope BehaviorValue,
		// @RequestParam("occurrenceStart")
		occurrenceStart BehaviorValue,
		// @RequestBody
		change BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/events/{id}")
	Delete(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestParam("scope")
		scope BehaviorValue,
		// @RequestParam("occurrenceStart")
		occurrenceStart BehaviorValue,
		// @RequestParam("idempotencyKey")
		idempotencyKey BehaviorValue,
	) (BehaviorValue, error)
}
