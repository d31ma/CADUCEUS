// RulesClient is the /rules route group. The notification centre counts the
// disabled rules; settings edits, orders and previews them.
// @HTTPClient("/rules")
type RulesClient interface {
	// @GetRequest
	Rules(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]InboxRule, error)

	// @PostRequest
	Create(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		rule BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/{id}")
	Update(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestBody
		rule BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/{id}")
	Delete(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/order")
	Order(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		order BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/preview")
	Preview(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		preview BehaviorValue,
	) (RulePreview, error)
}
