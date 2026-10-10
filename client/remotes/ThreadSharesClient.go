// @HTTPClient
type ThreadSharesClient interface {
	// @GetRequest("/thread-shares")
	List(
		// @RequestParam("emailId")
		emailId string,
		// @RequestHeader("Authorization")
		authorization string,
	) ([]ThreadShare, error)

	// @PostRequest("/thread-shares")
	Create(
		// @RequestBody
		share BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (CreatedShare, error)

	// @DeleteRequest("/thread-shares/{id}")
	Revoke(
		// @PathVariable
		id string,
		// @RequestHeader("Authorization")
		authorization string,
	) (BehaviorValue, error)
}
