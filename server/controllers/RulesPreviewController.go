import "net/http"

// @RequestRoute("/rules/preview")
// @SignedIn
type RulesPreviewController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	rulesHelper *RulesHelper
	// @Needs
	usersHelper *UsersHelper
}

// @PostRoute
func (controller *RulesPreviewController) PostPreview(request *http.Request) *http.Response {
	preview := RulesRoute{Access: controller.accessHelper, Rules: controller.rulesHelper, Users: controller.usersHelper}
	return preview.Preview(request)
}
