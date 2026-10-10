// @HTTPClient
type SendClient interface {
	// @GetRequest("/send/scheduled")
	Scheduled(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]ScheduledSend, error)

	// @GetRequest("/send/scheduled/{id}")
	ScheduledMessage(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable("id")
		id string,
	) (ScheduledSend, error)

	// @DeleteRequest("/send/scheduled/{id}")
	CancelScheduled(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable("id")
		id string,
	) (BehaviorValue, error)
}
