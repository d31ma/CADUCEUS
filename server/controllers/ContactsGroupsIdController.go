import "net/http"

// @RequestRoute("/contacts/groups/{id}")
// @SignedIn
type ContactsGroupsIdController struct {
	// @Needs
	contactsService *ContactsService
}

// @PutRoute
func (controller *ContactsGroupsIdController) PutGroup(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.contactsService.Groups().Update(id, request)
}

// @DeleteRoute
func (controller *ContactsGroupsIdController) DeleteGroup(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.contactsService.Groups().Delete(id, request)
}
