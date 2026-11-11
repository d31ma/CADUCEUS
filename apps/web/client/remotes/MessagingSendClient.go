// @HTTPClient("/api/v1/messaging")
type MessagingSendClient interface {
	// @PostRequest("/messages")
	Send(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)
}
