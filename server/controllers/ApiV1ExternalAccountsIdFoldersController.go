import "net/http"

// @RequestRoute("/api/v1/external-accounts/{id}/folders")
// @SignedIn
type ApiV1ExternalAccountsIdFoldersController struct {
	// @Needs
	accessHelper *AccessHelper
}

// GetFolders answers the provider folders of an account.
// @GetRoute
func (controller *ApiV1ExternalAccountsIdFoldersController) GetFolders(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Read(id, request, "accounts.folders")
}
