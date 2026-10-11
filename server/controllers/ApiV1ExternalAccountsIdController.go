import "net/http"

// @RequestRoute("/api/v1/external-accounts/{id}")
// @SignedIn
type ApiV1ExternalAccountsIdController struct {
	// @Needs
	accessHelper *AccessHelper
}

// GetAccount answers one external account of the caller.
// @GetRoute
func (controller *ApiV1ExternalAccountsIdController) GetAccount(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Get(id, request)
}

// PatchAccount pauses, resumes or updates the presentation of an account.
// @PatchRoute
func (controller *ApiV1ExternalAccountsIdController) PatchAccount(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Change(id, request)
}

// DeleteAccount deletes an account under its current revision.
// @DeleteRoute
func (controller *ApiV1ExternalAccountsIdController) DeleteAccount(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Remove(id, request)
}
