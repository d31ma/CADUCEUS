// @HTTPClient
type GithubClient interface {
	// @GetRequest("/github/profile")
	Profile(
		// @RequestParam("email")
		email string,
		// @RequestHeader("Authorization")
		authorization string,
	) (GithubLookup, error)
}
