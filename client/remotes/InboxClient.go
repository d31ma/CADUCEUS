// @HTTPClient
type InboxClient interface {
	// @GetRequest("/inbox/{id}")
	Read(
		// @PathVariable
		id string,
		// @RequestHeader("Authorization")
		authorization string,
	) (BehaviorValue, error)

	// @PutRequest("/inbox/{id}")
	Update(
		// @PathVariable
		id string,
		// @RequestBody
		patch BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (BehaviorValue, error)

	// @DeleteRequest("/inbox/{id}")
	Delete(
		// @PathVariable
		id string,
		// @RequestHeader("Authorization")
		authorization string,
	) (BehaviorValue, error)

	// @PutRequest("/inbox/{id}/spam")
	MarkSpam(
		// @PathVariable
		id string,
		// @RequestBody
		verdict BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (BehaviorValue, error)

	// @GetRequest("/inbox/{id}/source")
	Source(
		// @PathVariable
		id string,
		// @RequestHeader("Authorization")
		authorization string,
	) (MessageSource, error)

	// @GetRequest("/inbox/{id}/attachments/{attachmentId}")
	Attachment(
		// @PathVariable
		id string,
		// @PathVariable
		attachmentId string,
		// @RequestHeader("Authorization")
		authorization string,
	) (AttachmentContent, error)

	// @PostRequest("/inbox/{id}/images")
	Images(
		// @PathVariable
		id string,
		// @RequestBody
		request BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (ProxyImages, error)

	// @PostRequest("/inbox/actions")
	Act(
		// @RequestBody
		action BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (MailActionResult, error)
}
