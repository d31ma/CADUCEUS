// @HTTPClient
type ContactsClient interface {
	// @GetRequest("/contacts/suggestions")
	Suggestions(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("q")
		query string,
	) ([]ContactSuggestion, error)
}
