import "net/http"

// @RequestRoute("/api/v1/provider-discovery")
// @SignedIn
type ApiV1ProviderDiscoveryController struct {
	// @Needs
	accessHelper *AccessHelper
}

// PostDiscovery answers the provider settings discovered for an address.
// @PostRoute
func (controller *ApiV1ProviderDiscoveryController) PostDiscovery(request *http.Request) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Discover(request)
}
