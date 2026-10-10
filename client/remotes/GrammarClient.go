// @HTTPClient
type GrammarClient interface {
	// @PostRequest("/grammar/check")
	Check(
		// @RequestBody
		input BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (GrammarAnswer, error)
}
