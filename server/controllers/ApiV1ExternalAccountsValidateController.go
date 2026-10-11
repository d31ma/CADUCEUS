import "net/http"

// @RequestRoute("/api/v1/external-accounts/validate")
// @SignedIn
type ApiV1ExternalAccountsValidateController struct {
	// @Needs
	accessHelper *AccessHelper
}

// PostValidation checks an account's settings against the provider without storing it.
// @PostRoute
func (controller *ApiV1ExternalAccountsValidateController) PostValidation(request *http.Request) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Spread(request, "accounts.validate")
}
