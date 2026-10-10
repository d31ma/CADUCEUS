// @HTTPClient
type SearchClient interface {
	// @GetRequest("/search")
	Search(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("q")
		query string,
		// @RequestParam("limit")
		limit int,
		// @RequestParam("domain")
		domain string,
		// @RequestParam("account")
		account string,
	) ([]Email, error)
}
