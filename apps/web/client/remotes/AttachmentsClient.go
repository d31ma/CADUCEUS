// @HTTPClient("/api/v1/attachments")
type AttachmentsClient interface {
	// @PostRequest("")
	Create(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/{attachmentId}/complete")
	Complete(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable("attachmentId")
		attachmentId BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/{attachmentId}")
	Download(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable("attachmentId")
		attachmentId BehaviorValue,
	) (BehaviorValue, error)
}
