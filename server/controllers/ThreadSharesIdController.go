import "net/http"

// @RequestRoute("/thread-shares/{id}")
// @SignedIn
type ThreadSharesIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	threadSharingService *ThreadSharingService
}

// @DeleteRoute
func (controller *ThreadSharesIdController) DeleteShare(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return ThreadSharesRoute{Access: controller.accessHelper, Sharing: controller.threadSharingService}.Revoke(id, request)
}
