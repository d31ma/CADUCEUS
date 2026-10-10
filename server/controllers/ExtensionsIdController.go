import "net/http"

// @RequestRoute("/extensions/{id}")
// @SignedIn
type ExtensionsIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	extensionsHelper *ExtensionsHelper
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
	return controller.route().Enable(id, request)
}

// @DeleteRoute
func (controller *ExtensionsIdController) DeleteExtension(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Remove(id, request)
}

func (controller *ExtensionsIdController) route() ExtensionsRoute {
	return ExtensionsRoute{Access: controller.accessHelper, Extensions: controller.extensionsHelper}
}
