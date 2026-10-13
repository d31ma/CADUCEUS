import "net/http"

// @RequestRoute("/api/v1/external-accounts/oauth/microsoft/complete")
// @SignedIn
type ApiV1ExternalAccountsOauthMicrosoftCompleteController struct {
	// @Needs
	accessHelper *AccessHelper
}

// PostCompletion finishes connecting a Microsoft account.
// @PostRoute
func (controller *ApiV1ExternalAccountsOauthMicrosoftCompleteController) PostCompletion(request *http.Request) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.CompleteOAuth(request, "accounts.microsoft_oauth_complete")
}
