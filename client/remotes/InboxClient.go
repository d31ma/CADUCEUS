// @HTTPClient
type InboxClient interface {
	// List reads the message summaries of the signed-in mailbox. An empty
	// value leaves its filter out.
	//
	// @GetRequest("/inbox")
	List(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("read")
		read string,
		// @RequestParam("starred")
		starred string,
		// @RequestParam("domain")
		domain string,
		// @RequestParam("account")
		account string,
	) ([]Email, error)

	// @PostRequest("/inbox/actions")
	Act(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		action BehaviorValue,
	) (MailActionResult, error)

	// @PutRequest("/inbox/{id}/spam")
	MarkSpam(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable("id")
		id string,
		// @RequestBody
		verdict BehaviorValue,
	) (Email, error)
}
