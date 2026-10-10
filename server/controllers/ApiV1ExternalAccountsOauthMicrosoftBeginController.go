import "net/http"

// @RequestRoute("/api/v1/external-accounts/oauth/microsoft/begin")
// @SignedIn
type ApiV1ExternalAccountsOauthMicrosoftBeginController struct {
	// @Needs
	accessHelper *AccessHelper
}

// PostBegin starts connecting a Microsoft account.
// @PostRoute
func (controller *ApiV1ExternalAccountsOauthMicrosoftBeginController) PostBegin(request *http.Request) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Spread(request, "accounts.microsoft_oauth_begin")
}
