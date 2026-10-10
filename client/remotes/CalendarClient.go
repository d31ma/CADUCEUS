// @HTTPClient
type CalendarClient interface {
	// @PostRequest("/calendar/invites/respond")
	RespondToInvite(
		// @RequestBody
		response BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (BehaviorValue, error)
}
