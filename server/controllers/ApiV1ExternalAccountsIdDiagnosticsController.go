import "net/http"

// @RequestRoute("/api/v1/external-accounts/{id}/diagnostics")
// @SignedIn
type ApiV1ExternalAccountsIdDiagnosticsController struct {
	// @Needs
	accessHelper *AccessHelper
}

// GetDiagnostics answers the synchronization state of an account.
// @GetRoute
func (controller *ApiV1ExternalAccountsIdDiagnosticsController) GetDiagnostics(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Read(id, request, "accounts.diagnostics")
}
