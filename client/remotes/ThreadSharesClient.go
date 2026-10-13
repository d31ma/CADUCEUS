// ThreadSharesClient is the /thread-shares route group. The public share page
// reads one expiring conversation and its attachments with no credentials.
// @HTTPClient({"path":"/thread-shares","setup":"SameOriginSetup"})
type ThreadSharesClient interface {
	// @GetRequest("/public/{token}")
	Thread(
		// @PathVariable
		token string,
	) (SharedThread, error)

	// @GetRequest("/public/{token}/attachments/{id}")
	Attachment(
		// @PathVariable
		token string,
		// @PathVariable
		id string,
	) (SharedAttachmentContent, error)

	// @GetRequest
	List(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("emailId")
		emailID string,
	) ([]ThreadShare, error)

	// @PostRequest
	Create(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		share BehaviorValue,
	) (CreatedShare, error)

	// @DeleteRequest("/{id}")
	Revoke(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
	) (BehaviorValue, error)
}
