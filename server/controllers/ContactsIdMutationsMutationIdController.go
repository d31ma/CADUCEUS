import "net/http"

// @RequestRoute("/contacts/{id}/mutations/{mutation_id}")
// @SignedIn
type ContactsIdMutationsMutationIdController struct {
	// @Needs
	contactsService *ContactsService
}

// @GetRoute
func (controller *ContactsIdMutationsMutationIdController) GetMutation(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.contactsService.Item().Mutation(id, request)
}
