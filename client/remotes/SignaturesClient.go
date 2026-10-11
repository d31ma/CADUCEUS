// SignaturesClient is the /signatures route group. The composer offers the
// mailbox's signatures.
// @HTTPClient("/signatures")
type SignaturesClient interface {
	// @GetRequest
	List(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]ComposeSignature, error)
}
