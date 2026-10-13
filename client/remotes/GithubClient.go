// GithubClient is the /github route group: the opted-in public profile of a
// message's participant.
// @HTTPClient({"path":"/github","setup":"SameOriginSetup"})
type GithubClient interface {
	// @GetRequest("/profile")
	Profile(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("email")
		email string,
	) (GithubLookup, error)
}
