// @HTTPClient
type SendClient interface {
	// @PostRequest("/send")
	Send(
		// @RequestBody
		message BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (BehaviorValue, error)
}
