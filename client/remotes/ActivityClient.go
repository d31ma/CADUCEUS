// @HTTPClient
type ActivityClient interface {
	// @GetRequest("/activity")
	Report(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("type")
		kind string,
		// @RequestParam("range")
		period string,
		// @RequestParam("limit")
		limit int,
	) (ActivityReport, error)
}
