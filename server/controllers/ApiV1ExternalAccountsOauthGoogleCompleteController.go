import "net/http"

// @RequestRoute("/api/v1/external-accounts/oauth/google/complete")
// @SignedIn
type ApiV1ExternalAccountsOauthGoogleCompleteController struct {
	// @Needs
	accessHelper *AccessHelper
}

// PostCompletion finishes connecting a Google account.
// @PostRoute
func (controller *ApiV1ExternalAccountsOauthGoogleCompleteController) PostCompletion(request *http.Request) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.CompleteOAuth(request, "accounts.google_oauth_complete")
}
