import "net/http"

// @RequestRoute("/api/v1/external-accounts/{id}/reconnect")
// @SignedIn
type ApiV1ExternalAccountsIdReconnectController struct {
	// @Needs
	accessHelper *AccessHelper
}

// PostReconnection replaces the credentials of an account.
// @PostRoute
func (controller *ApiV1ExternalAccountsIdReconnectController) PostReconnection(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Reconnect(id, request)
}
