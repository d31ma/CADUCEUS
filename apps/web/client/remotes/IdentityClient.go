// @HTTPClient("/api/v1/identity")
type IdentityClient interface {
	// @PostRequest("/session")
	SignIn(
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/session/refresh")
	Refresh(
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/session")
	SignOut(
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)
}
