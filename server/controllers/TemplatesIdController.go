import "net/http"

// @RequestRoute("/templates/{id}")
// @SignedIn
type TemplatesIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	templatesHelper *TemplatesHelper
}

func (controller *TemplatesIdController) templates() TemplatesRoute {
	return TemplatesRoute{Access: controller.accessHelper, Templates: controller.templatesHelper}
}

// @PutRoute
func (controller *TemplatesIdController) PutTemplate(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return TemplatesRoute{Access: controller.accessHelper, Templates: controller.templatesHelper}.Update(id, request)
}

// @DeleteRoute
func (controller *TemplatesIdController) DeleteTemplate(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	templates := controller.templates()
	return templates.Delete(id, request)
}
