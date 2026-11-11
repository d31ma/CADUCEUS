// @HTTPClient("/api/v1/messaging")
type MessagingClient interface {
	// @GetRequest("/conversations")
	Conversations(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("cursor")
		cursor string,
		// @RequestParam("limit")
		limit int,
	) (ConversationPage, error)

	// @GetRequest("/conversations/{conversationId}/messages")
	Messages(
		// @PathVariable("conversationId")
		conversationId string,
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("cursor")
		cursor string,
		// @RequestParam("limit")
		limit int,
	) (MessagePage, error)

	// @PatchRequest("/messages/{messageId}")
	Edit(
		// @PathVariable("messageId")
		messageId BehaviorValue,
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/messages/{messageId}")
	Delete(
		// @PathVariable("messageId")
		messageId BehaviorValue,
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/messages/{messageId}/reactions/{emoji}")
	AddReaction(
		// @PathVariable("messageId")
		messageId BehaviorValue,
		// @PathVariable("emoji")
		emoji BehaviorValue,
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/messages/{messageId}/reactions/{emoji}")
	RemoveReaction(
		// @PathVariable("messageId")
		messageId BehaviorValue,
		// @PathVariable("emoji")
		emoji BehaviorValue,
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/conversations/{conversationId}/read")
	MarkRead(
		// @PathVariable("conversationId")
		conversationId BehaviorValue,
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/messages/{messageId}/original")
	Original(
		// @PathVariable("messageId")
		messageId string,
		// @RequestHeader("Authorization")
		authorization string,
	) (MessageDownload, error)
}
