// ActivityClient is the /activity route group. The shell reads how many
// recipient events arrived since the Activity page was last viewed.
// @HTTPClient("/activity")
type ActivityClient interface {
	// @GetRequest
	Unseen(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("range")
		span string,
		// @RequestParam("limit")
		limit string,
		// @RequestParam("since")
		since string,
	) (ActivitySummary, error)

}
