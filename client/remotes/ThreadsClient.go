// @HTTPClient
type ThreadsClient interface {
	// @GetRequest("/threads")
	Find(
		// @RequestParam("emailId")
		emailId string,
		// @RequestHeader("Authorization")
		authorization string,
	) ([]ThreadRecord, error)
}
