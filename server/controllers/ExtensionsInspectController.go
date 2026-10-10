import "net/http"

// @RequestRoute("/extensions/inspect")
// @SignedIn
type ExtensionsInspectController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	extensionsHelper *ExtensionsHelper
}

// @PostRoute
func (controller *ExtensionsInspectController) PostInspection(request *http.Request) *http.Response {
	return ExtensionsRoute{Access: controller.accessHelper, Extensions: controller.extensionsHelper}.Inspect(request)
}
