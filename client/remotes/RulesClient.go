// RulesClient is the /rules route group. The notification centre counts the
// disabled rules.
// @HTTPClient("/rules")
type RulesClient interface {
	// @GetRequest
	All(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]RuleSummary, error)
}
