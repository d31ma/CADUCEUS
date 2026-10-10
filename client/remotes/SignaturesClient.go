// @HTTPClient
type SignaturesClient interface {
	// @GetRequest("/signatures")
	List(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]ComposeSignature, error)
}
