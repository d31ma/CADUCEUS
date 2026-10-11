// @HTTPClient({"setup":"SameOriginSetup"})
type ExtensionsClient interface {
	// @GetRequest("/extensions")
	Extensions(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]InstalledExtension, error)

	// @PostRequest("/extensions/inspect")
	Inspect(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (InstalledExtension, error)

	// @PostRequest("/extensions")
	Install(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (InstalledExtension, error)

	// @PutRequest("/extensions/{id}")
	Update(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/extensions/{id}")
	Remove(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
	) (BehaviorValue, error)
}
