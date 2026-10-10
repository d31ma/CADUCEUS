// @HTTPClient
type SearchClient interface {
	// @GetRequest("/search")
	Messages(
		// @RequestParam("q")
		query string,
		// @RequestHeader("Authorization")
		authorization string,
	) ([]RelatedMessage, error)
}
