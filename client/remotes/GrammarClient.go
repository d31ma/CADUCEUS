// @HTTPClient
type GrammarClient interface {
	// @PostRequest("/grammar/check")
	Check(
		// @RequestBody
		input GrammarInput,
		// @RequestHeader("Authorization")
		authorization string,
	) (GrammarAnswer, error)
}
