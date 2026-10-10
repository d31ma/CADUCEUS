import "net/http"

// @RequestRoute("/contacts/groups")
// @SignedIn
type ContactsGroupsController struct {
	// @Needs
	contactsService *ContactsService
}

// @GetRoute
func (controller *ContactsGroupsController) GetGroups(request *http.Request) *http.Response {
	return controller.contactsService.Groups().List(request)
}

// @PostRoute
func (controller *ContactsGroupsController) PostGroup(request *http.Request) *http.Response {
	return controller.contactsService.Groups().Create(request)
}
