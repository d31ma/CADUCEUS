import "net/http"

// @RequestRoute("/suppressed")
// @SignedIn
type SuppressedController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	suppressedHelper *SuppressedHelper
}

// @GetRoute
func (controller *SuppressedController) GetSuppressed(request *http.Request) *http.Response {
	return SuppressedRoute{Access: controller.accessHelper, Suppressed: controller.suppressedHelper}.List(request)
}

// @PostRoute
func (controller *SuppressedController) PostSuppressed(request *http.Request) *http.Response {
	return SuppressedRoute{Access: controller.accessHelper, Suppressed: controller.suppressedHelper}.Create(request)
}
