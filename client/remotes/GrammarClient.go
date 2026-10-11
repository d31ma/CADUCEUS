// GrammarClient is the /grammar route group. The composer checks its draft
// text with the server's local grammar checker.
// @HTTPClient("/grammar")
type GrammarClient interface {
	// @PostRequest("/check")
	Check(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		input BehaviorValue,
	) (GrammarAnswer, error)
}
