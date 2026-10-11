// @HTTPClient("/contacts")
type ContactsClient interface {
	// @GetRequest("/suggestions")
	Suggestions(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("q")
		query string,
	) ([]ContactSuggestion, error)

	// @GetRequest
	Search(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("q")
		query string,
	) ([]ParticipantContact, error)
}
