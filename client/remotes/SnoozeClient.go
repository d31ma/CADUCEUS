// SnoozeClient is the /snooze route group. The shell undoes a snooze that the
// reader left pending across a navigation.
// @HTTPClient("/snooze")
type SnoozeClient interface {
	// @DeleteRequest("/{id}")
	Cancel(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest
	Snooze(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		snooze BehaviorValue,
	) (SnoozeResult, error)
}
