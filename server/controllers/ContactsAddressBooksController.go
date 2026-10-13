import "net/http"

// @RequestRoute("/contacts/address-books")
// @SignedIn
type ContactsAddressBooksController struct {
	// @Needs
	contactsService *ContactsService
}

// @GetRoute
func (controller *ContactsAddressBooksController) GetAddressBooks(request *http.Request) *http.Response {
	return controller.contactsService.Collection().AddressBooks(request)
}
