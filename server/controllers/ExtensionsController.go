import "net/http"

// @RequestRoute("/extensions")
// @SignedIn
type ExtensionsController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	extensionsHelper *ExtensionsHelper
}

func (controller *ExtensionsController) route() ExtensionsRoute {
	return ExtensionsRoute{Access: controller.accessHelper, Extensions: controller.extensionsHelper}
}

// @GetRoute
func (controller *ExtensionsController) GetExtensions(request *http.Request) *http.Response {
	return controller.route().List(request)
}

// @PostRoute
func (controller *ExtensionsController) PostExtension(request *http.Request) *http.Response {
	return controller.route().Install(request)
}
