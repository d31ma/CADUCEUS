import "net/http"

// @RequestRoute("/drafts/{id}")
// @SignedIn
type DraftsIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	draftService *DraftService
	// @Needs
	usersHelper *UsersHelper
}

// @GetRoute
func (controller *DraftsIdController) GetDraft(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return DraftsRoute{Access: controller.accessHelper, Drafts: controller.draftService, Users: controller.usersHelper}.Get(id, request)
}

// @PutRoute
func (controller *DraftsIdController) PutDraft(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return DraftsRoute{Access: controller.accessHelper, Drafts: controller.draftService, Users: controller.usersHelper}.Update(id, request)
}

// @DeleteRoute
func (controller *DraftsIdController) DeleteDraft(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return DraftsRoute{Access: controller.accessHelper, Drafts: controller.draftService, Users: controller.usersHelper}.Discard(id, request)
}
