import "net/http"

// @RequestRoute("/activity")
// @SignedIn
type ActivityController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	categoryMappingService *CategoryMappingService
	// @Needs
	trackingHelper *TrackingHelper
	// @Needs
	usersHelper *UsersHelper
}

// @GetRoute
func (controller *ActivityController) GetActivity(request *http.Request) *http.Response {
	return ActivityRoute{
		Parts: MailboxParts{Access: controller.accessHelper, Categories: controller.categoryMappingService},
		Users: controller.usersHelper, Tracking: controller.trackingHelper,
	}.Feed(request)
}
