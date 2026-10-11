// InboxClient is the /inbox route group. The shell counts the mailbox for the
// sidebar badges and applies the mailbox actions it owns: moves, undo, redo
// and spam reports.
// @HTTPClient("/inbox")
type InboxClient interface {
	// @GetRequest
	Summaries(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]MailSummary, error)

	// @PostRequest("/actions")
	Apply(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		action BehaviorValue,
	) (MailActionResult, error)

	// @PostRequest("/actions/{id}/undo")
	Undo(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (MailActionResult, error)

	// @PostRequest("/actions/{id}/redo")
	Redo(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (MailActionResult, error)

	// @PutRequest("/{id}/spam")
	MarkSpam(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestBody
		mark BehaviorValue,
	) (BehaviorValue, error)
}
