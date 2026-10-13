import "net/http"

// @RequestRoute("/api/v1/external-accounts/oauth/google/begin")
// @SignedIn
type ApiV1ExternalAccountsOauthGoogleBeginController struct {
	// @Needs
	accessHelper *AccessHelper
}

// PostBegin starts connecting a Google account.
// @PostRoute
func (controller *ApiV1ExternalAccountsOauthGoogleBeginController) PostBegin(request *http.Request) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Spread(request, "accounts.google_oauth_begin")
}
