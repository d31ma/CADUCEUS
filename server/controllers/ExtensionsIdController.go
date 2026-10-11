import "net/http"

// @RequestRoute("/extensions/{id}")
// @SignedIn
type ExtensionsIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	extensionsHelper *ExtensionsHelper
}

func (controller *ExtensionsIdController) route() ExtensionsRoute {
	return ExtensionsRoute{Access: controller.accessHelper, Extensions: controller.extensionsHelper}
}

// @GetRoute
func (controller *ExtensionsIdController) GetExtension(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Read(id, request)
}

// @PutRoute
func (controller *ExtensionsIdController) PutExtension(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	extensions := controller.route()
	return extensions.Enable(id, request)
}

// @DeleteRoute
func (controller *ExtensionsIdController) DeleteExtension(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Remove(id, request)
}
