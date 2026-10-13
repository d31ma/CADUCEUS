import "net/http"

// @RequestRoute("/api/v1/external-accounts/{id}/retry")
// @SignedIn
type ApiV1ExternalAccountsIdRetryController struct {
	// @Needs
	accessHelper *AccessHelper
}

// PostRetry schedules another synchronization of an account.
// @PostRoute
func (controller *ApiV1ExternalAccountsIdRetryController) PostRetry(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Retry(id, request)
}
