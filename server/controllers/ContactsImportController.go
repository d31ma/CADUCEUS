import "net/http"

// @RequestRoute("/contacts/import")
// @SignedIn
type ContactsImportController struct {
	// @Needs
	contactsService *ContactsService
}

// @PostRoute
func (controller *ContactsImportController) PostImport(request *http.Request) *http.Response {
	return controller.contactsService.Collection().Import(request)
}
