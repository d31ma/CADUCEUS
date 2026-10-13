// DraftsClient is the /drafts route group. The composer loads, autosaves and
// discards the draft it is writing.
// @HTTPClient({"path":"/drafts","setup":"SameOriginSetup"})
type DraftsClient interface {
	// @GetRequest("/{id}")
	Draft(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (Draft, error)

	// @PostRequest
	Create(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		draft BehaviorValue,
	) (Draft, error)

	// @PutRequest("/{id}")
	Update(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		draft BehaviorValue,
	) (Draft, error)

	// @DeleteRequest("/{id}")
	Discard(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (DraftDiscarded, error)
}
