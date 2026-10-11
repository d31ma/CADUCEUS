import "net/http"

// @RequestRoute("/drafts")
// @SignedIn
type DraftsController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	draftService *DraftService
	// @Needs
	usersHelper *UsersHelper
}

// @PostRoute
func (controller *DraftsController) PostDraft(request *http.Request) *http.Response {
	return DraftsRoute{Access: controller.accessHelper, Drafts: controller.draftService, Users: controller.usersHelper}.Create(request)
}
