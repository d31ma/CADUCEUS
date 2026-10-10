import "net/http"

// @RequestRoute("/contacts/suggestions")
// @SignedIn
type ContactsSuggestionsController struct {
	// @Needs
	contactsService *ContactsService
}

// @GetRoute
func (controller *ContactsSuggestionsController) GetSuggestions(request *http.Request) *http.Response {
	return controller.contactsService.Collection().Suggestions(request)
}
