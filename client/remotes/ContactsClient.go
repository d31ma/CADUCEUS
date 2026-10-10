// @HTTPClient
type ContactsClient interface {
	// @GetRequest("/contacts/suggestions")
	Suggestions(
		// @RequestParam("q")
		query string,
		// @RequestHeader("Authorization")
		authorization string,
	) ([]RecipientSuggestion, error)
}
