// @HTTPClient("/api/v1/directory")
type DirectoryClient interface {
	// @PostRequest("/recipients/resolve")
	Resolve(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)
}
