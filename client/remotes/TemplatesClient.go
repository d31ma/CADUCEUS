// TemplatesClient is the /templates route group. The composer lists, inserts
// and saves message templates.
// @HTTPClient("/templates")
type TemplatesClient interface {
	// @GetRequest
	List(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]ComposeTemplate, error)

	// @PostRequest
	Save(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		template BehaviorValue,
	) (SavedTemplate, error)
}
