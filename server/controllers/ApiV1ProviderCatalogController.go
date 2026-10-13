import "net/http"

// @RequestRoute("/api/v1/provider-catalog")
// @SignedIn
type ApiV1ProviderCatalogController struct {
	// @Needs
	accessHelper *AccessHelper
}

// GetProviders answers {providers}: the provider catalog.
// @GetRoute
func (controller *ApiV1ProviderCatalogController) GetProviders(request *http.Request) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Providers(request)
}
