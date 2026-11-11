// @HTTPClient("/api/v1/messaging")
type ThreadClient interface {
	// @GetRequest("/conversations/{conversationId}/messages")
	Replies(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable("conversationId")
		conversationId BehaviorValue,
		// @RequestParam("threadRootId")
		threadRootId BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/conversations/{conversationId}/messages")
	OlderReplies(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable("conversationId")
		conversationId BehaviorValue,
		// @RequestParam("threadRootId")
		threadRootId BehaviorValue,
		// @RequestParam("cursor")
		cursor BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/messages/{messageId}")
	Message(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable("messageId")
		messageId BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/messages")
	Send(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)
}
