// @HTTPClient
type RemindersClient interface {
	// @GetRequest("/reminders")
	List(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]Reminder, error)

	// @DeleteRequest("/reminders")
	Dismiss(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		dismissal BehaviorValue,
	) (ReminderDismissed, error)
}
