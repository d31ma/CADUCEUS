// @HTTPClient({"setup":"SameOriginSetup"})
type SignaturesClient interface {
	// @GetRequest("/signatures")
	Signatures(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]MailSignature, error)

	// @PostRequest("/signatures")
	Create(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (MailSignature, error)

	// @PutRequest("/signatures/{id}")
	Update(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		body BehaviorValue,
	) (MailSignature, error)

	// @DeleteRequest("/signatures/{id}")
	Delete(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
	) (BehaviorValue, error)
}
