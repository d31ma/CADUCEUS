// @HTTPClient("/reminders")
type RemindersClient interface {
	// @GetRequest
	List(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]Reminder, error)

	// @DeleteRequest
	Dismiss(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		dismissal BehaviorValue,
	) (ReminderDismissed, error)
}
