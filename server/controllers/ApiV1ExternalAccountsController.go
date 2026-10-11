import "net/http"

// @RequestRoute("/api/v1/external-accounts")
// @SignedIn
type ApiV1ExternalAccountsController struct {
	// @Needs
	accessHelper *AccessHelper
}

// GetAccounts answers {accounts}: the caller's external accounts.
// @GetRoute
func (controller *ApiV1ExternalAccountsController) GetAccounts(request *http.Request) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.List(request)
}

// PostAccount validates and stores a new external account.
// @PostRoute
func (controller *ApiV1ExternalAccountsController) PostAccount(request *http.Request) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Spread(request, "accounts.create")
}
