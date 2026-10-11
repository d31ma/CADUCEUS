// @HTTPClient("/search")
type SearchClient interface {
	// @GetRequest
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
