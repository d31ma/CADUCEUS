import "net/http"

// @RequestRoute("/contacts/export")
// @SignedIn
type ContactsExportController struct {
	// @Needs
	contactsService *ContactsService
}

// @PostRoute
func (controller *ContactsExportController) PostExport(request *http.Request) *http.Response {
	return controller.contactsService.Collection().Export(request)
}
