import "net/http"

// @RequestRoute("/api/v1/external-accounts/{id}/messages")
// @SignedIn
type ApiV1ExternalAccountsIdMessagesController struct {
	// @Needs
	accessHelper *AccessHelper
}

// GetMessages answers the indexed messages of one folder.
// @GetRoute
func (controller *ApiV1ExternalAccountsIdMessagesController) GetMessages(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Messages(id, request)
}
