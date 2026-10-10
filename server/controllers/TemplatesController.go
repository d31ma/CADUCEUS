import "net/http"

// @RequestRoute("/templates")
// @SignedIn
type TemplatesController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	templatesHelper *TemplatesHelper
}

// @PostRoute
func (controller *TemplatesController) PostTemplate(request *http.Request) *http.Response {
	return controller.route().Create(request)
}

// @DeleteRoute
func (controller *TemplatesController) DeleteTemplate(request *http.Request) *http.Response {
	return controller.route().DeleteNamed(request)
}

// @GetRoute
func (controller *TemplatesController) GetTemplates(request *http.Request) *http.Response {
	return controller.route().List(request)
}

func (controller *TemplatesController) route() TemplatesRoute {
	return TemplatesRoute{Access: controller.accessHelper, Templates: controller.templatesHelper}
}
