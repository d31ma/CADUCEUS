import "net/http"

// @RequestRoute("/api/v1/external-accounts/{id}/mutations")
// @SignedIn
type ApiV1ExternalAccountsIdMutationsController struct {
	// @Needs
	accessHelper *AccessHelper
}

// PostMutation queues one provider mutation.
// @PostRoute
func (controller *ApiV1ExternalAccountsIdMutationsController) PostMutation(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Mutate(id, request)
}
