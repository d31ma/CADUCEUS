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

	// @PostRequest("/password")
	ChangePassword(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/totp/setup")
	SetUpTwoStep(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/totp/confirm")
	ConfirmTwoStep(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/totp")
	RemoveTwoStep(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/totp")
	TwoStepStatus(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)
}
