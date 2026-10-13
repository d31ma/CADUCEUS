// InboxClient is the /inbox route group. The shell counts the mailbox for the
// sidebar badges and applies the mailbox actions it owns: moves, undo, redo
// and spam reports.
// @HTTPClient({"path":"/inbox","setup":"SameOriginSetup"})
type InboxClient interface {
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

	// List reads the message summaries the mailbox list shows. An empty value
	// leaves its filter out.
	//
	// @GetRequest
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

	// Read is the whole message the reader opens.
	//
	// @GetRequest("/{id}")
	Read(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (EmailDetail, error)

	// @PutRequest("/{id}")
	Update(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestBody
		patch BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/{id}")
	Delete(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/{id}/source")
	Source(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (MessageSource, error)

	// @GetRequest("/{id}/attachments/{attachmentId}")
	Attachment(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @PathVariable
		attachmentId string,
	) (AttachmentContent, error)

	// Images reads remote images through the server's privacy proxy.
	//
	// @PostRequest("/{id}/images")
	Images(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		request BehaviorValue,
	) (ProxyImages, error)
}
