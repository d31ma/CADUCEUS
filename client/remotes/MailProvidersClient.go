// MailProvidersClient reads the catalog of outside mail providers and
// discovers the server settings of an address.
// @HTTPClient({"path":"/api/v1","setup":"SameOriginSetup"})
type MailProvidersClient interface {
	// @GetRequest("/provider-catalog")
	Catalog(
		// @RequestHeader("Authorization")
		authorization string,
	) (ProviderCatalog, error)

	// @PostRequest("/provider-discovery")
	Discover(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		address BehaviorValue,
	) (DiscoveredSettings, error)
}
