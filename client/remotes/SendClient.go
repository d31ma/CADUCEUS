// SendClient is the /send route group. The shell undoes a send that the
// composer left pending across a navigation.
// @HTTPClient({"path":"/send","setup":"SameOriginSetup"})
type SendClient interface {
	// @PostRequest("/undo")
	Undo(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		undo BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/scheduled")
	Scheduled(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]ScheduledSend, error)

	// @GetRequest("/scheduled/{id}")
	ScheduledMessage(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (ScheduledSend, error)

	// @DeleteRequest("/scheduled/{id}")
	CancelScheduled(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (ScheduledCancelled, error)
}
