// @HTTPClient
type ExternalAccountsClient interface {
	// @GetRequest("/api/v1/external-accounts")
	List(
		// @RequestHeader("Authorization")
		authorization string,
	) (ExternalAccountList, error)
}
