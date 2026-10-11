import "net/http"

// @RequestRoute("/inbox/{id}/images")
// @SignedIn
type InboxIdImagesController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	rateLimitService *RateLimitService
}

// @PostRoute
func (controller *InboxIdImagesController) PostImages(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return RemoteImagesRoute{
		Parts:      MailboxParts{Access: controller.accessHelper, Emails: controller.emailsHelper},
		RateLimits: controller.rateLimitService,
	}.Fetch(id, request)
}
