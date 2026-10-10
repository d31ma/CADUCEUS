// @HTTPClient
type DraftsClient interface {
	// @DeleteRequest("/drafts/{id}")
	Discard(
		// @PathVariable
		id string,
		// @RequestHeader("Authorization")
		authorization string,
	) (DraftDiscarded, error)
}
