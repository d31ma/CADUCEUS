import "net/http"

// @RequestRoute("/contacts/{id}")
// @SignedIn
type ContactsIdController struct {
	// @Needs
	contactsService *ContactsService
}

// @GetRoute
func (controller *ContactsIdController) GetContact(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.contactsService.Item().Get(id, request)
}

// @PutRoute
func (controller *ContactsIdController) PutContact(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.contactsService.Item().Update(id, request)
}

// @DeleteRoute
func (controller *ContactsIdController) DeleteContact(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.contactsService.Item().Delete(id, request)
}
