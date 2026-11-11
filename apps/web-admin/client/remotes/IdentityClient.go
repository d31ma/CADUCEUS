// @HTTPClient("/api/v1/identity")
type IdentityClient interface {
	// @PostRequest("/session")
	SignIn(
		// @RequestBody
		request BehaviorValue,
	) (Session, error)

	// @PostRequest("/session/refresh")
	Refresh(
		// @RequestBody
		request BehaviorValue,
	) (Session, error)

	// @PostRequest("/session/domain")
	SwitchDomain(
		// @RequestBody
		request BehaviorValue,
	) (Session, error)

	// @DeleteRequest("/session")
	SignOut(
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)
}
