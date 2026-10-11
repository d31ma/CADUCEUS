// SendClient is the /send route group. The composer sends and schedules,
// and the shell undoes a send that the composer left pending across a
// navigation.
// @HTTPClient("/send")
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

	// @PostRequest
	Send(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		message BehaviorValue,
	) (SendAnswer, error)

	// @PutRequest("/scheduled/{id}")
	Reschedule(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		message BehaviorValue,
	) (SendAnswer, error)
}
