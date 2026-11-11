// @HTTPClient("/api/v1/channels")
type ChannelsClient interface {
	// @GetRequest("")
	Channels(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]Channel, error)

	// @PatchRequest("/{channelId}")
	UpdateChannel(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		channelId string,
		// @RequestBody
		request BehaviorValue,
	) (Channel, error)
}
