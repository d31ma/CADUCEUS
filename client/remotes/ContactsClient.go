// @HTTPClient
type ContactsClient interface {
	// @GetRequest("/contacts")
	Search(
		// @RequestParam("q")
		query string,
		// @RequestHeader("Authorization")
		authorization string,
	) ([]ParticipantContact, error)
}
