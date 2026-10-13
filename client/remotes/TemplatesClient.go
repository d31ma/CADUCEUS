// @HTTPClient({"setup":"SameOriginSetup"})
type TemplatesClient interface {
	// @GetRequest("/templates")
	Templates(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]MessageTemplate, error)

	// @PostRequest("/templates")
	Create(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (MessageTemplate, error)

	// @PutRequest("/templates/{id}")
	Update(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		body BehaviorValue,
	) (MessageTemplate, error)

	// @DeleteRequest("/templates/{id}")
	Delete(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
	) (BehaviorValue, error)
}
