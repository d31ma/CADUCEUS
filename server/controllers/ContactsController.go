import "net/http"

// @RequestRoute("/contacts")
// @SignedIn
type ContactsController struct {
	// @Needs
	contactsService *ContactsService
}

// @GetRoute
func (controller *ContactsController) GetContacts(request *http.Request) *http.Response {
	return controller.contactsService.Collection().List(request)
}

// @PostRoute
func (controller *ContactsController) PostContact(request *http.Request) *http.Response {
	return controller.contactsService.Collection().Create(request)
}
