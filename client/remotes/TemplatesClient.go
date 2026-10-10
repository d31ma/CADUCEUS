// @HTTPClient
type TemplatesClient interface {
	// @GetRequest("/templates")
	List(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]ComposeTemplate, error)

	// @PostRequest("/templates")
	Save(
		// @RequestBody
		template BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (SavedTemplate, error)
}
