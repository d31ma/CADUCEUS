// @HTTPClient
type RulesClient interface {
	// @GetRequest("/rules")
	Rules(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]InboxRule, error)

	// @PostRequest("/rules")
	Create(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/rules/{id}")
	Update(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/rules/{id}")
	Delete(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/rules/order")
	Order(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/rules/preview")
	Preview(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (RulePreview, error)
}
