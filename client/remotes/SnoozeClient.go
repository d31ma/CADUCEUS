// @HTTPClient
type SnoozeClient interface {
	// @PostRequest("/snooze")
	Snooze(
		// @RequestBody
		snooze BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (SnoozeResult, error)
}
