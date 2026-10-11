import "net/http"

// @RequestRoute("/api/v1/external-accounts/{id}/mutations/{mutation_id}")
// @SignedIn
type ApiV1ExternalAccountsIdMutationsMutationIdController struct {
	// @Needs
	accessHelper *AccessHelper
}

// GetMutation answers one queued provider mutation.
// @GetRoute
func (controller *ApiV1ExternalAccountsIdMutationsMutationIdController) GetMutation(
	// @FromPath
	id string,
	// @FromPath("mutation_id")
	mutationId string,
	request *http.Request,
) *http.Response {
	return ExternalAccountsRoute{Access: controller.accessHelper}.Mutation(id, mutationId, request)
}
