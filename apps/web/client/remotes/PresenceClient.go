// @HTTPClient("/api/v1/realtime")
type PresenceClient interface {
	// @PutRequest("/presence")
	Announce(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/presence")
	Find(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestParam("memberIds")
		memberIds BehaviorValue,
	) (BehaviorValue, error)
}
