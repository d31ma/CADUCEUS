// @HTTPClient("/api/v1")
type SearchClient interface {
	// @GetRequest("/search")
	Search(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestParam("q")
		query BehaviorValue,
		// @RequestParam("conversationId")
		conversationId BehaviorValue,
		// @RequestParam("author")
		author BehaviorValue,
		// @RequestParam("from")
		from BehaviorValue,
		// @RequestParam("to")
		to BehaviorValue,
		// @RequestParam("hasAttachment")
		hasAttachment BehaviorValue,
		// @RequestParam("isEmail")
		isEmail BehaviorValue,
		// @RequestParam("cursor")
		cursor BehaviorValue,
		// @RequestParam("limit")
		limit BehaviorValue,
	) (BehaviorValue, error)
}
