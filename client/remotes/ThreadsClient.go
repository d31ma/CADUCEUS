// @HTTPClient("/threads")
type ThreadsClient interface {
	// List reads conversations: the ones matching the query, or with an empty
	// query the ones in the folder.
	//
	// @GetRequest
	List(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("q")
		query string,
		// @RequestParam("folder")
		folder string,
		// @RequestParam("domain")
		domain string,
		// @RequestParam("account")
		account string,
	) ([]MailThread, error)
}
