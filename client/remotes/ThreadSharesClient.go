// ThreadSharesClient is the /thread-shares route group. The public share page
// reads one expiring conversation and its attachments with no credentials.
// @HTTPClient("/thread-shares")
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
}
