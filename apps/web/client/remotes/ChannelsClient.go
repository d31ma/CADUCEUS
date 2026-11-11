// @HTTPClient("/api/v1")
type ChannelsClient interface {
	// @GetRequest("/channels")
	List(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)
}
