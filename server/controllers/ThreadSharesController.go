import "net/http"

// @RequestRoute("/thread-shares")
// @SignedIn
type ThreadSharesController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	threadSharingService *ThreadSharingService
}

// @PostRoute
func (controller *ThreadSharesController) PostShare(request *http.Request) *http.Response {
	shares := ThreadSharesRoute{Access: controller.accessHelper, Sharing: controller.threadSharingService}
	return shares.Create(request)
}

// @GetRoute
func (controller *ThreadSharesController) GetShares(request *http.Request) *http.Response {
	shares := ThreadSharesRoute{Access: controller.accessHelper, Sharing: controller.threadSharingService}
	return shares.List(request)
}
