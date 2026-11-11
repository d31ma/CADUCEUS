// @HTTPClient("/api/v1/identity")
type IdentityClient interface {
	// @GetRequest("/config")
	Config() (BehaviorValue, error)

	// @GetRequest("/me")
	Me(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/token")
	Token(
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/refresh")
	Refresh(
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)
}
