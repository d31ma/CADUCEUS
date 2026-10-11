import "net/http"

// @RequestRoute("/categories")
// @SignedIn
type CategoriesController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	categoryMappingService *CategoryMappingService
}

// @GetRoute
func (controller *CategoriesController) GetCategories(request *http.Request) *http.Response {
	return controller.route().List(request)
}

// @PostRoute
func (controller *CategoriesController) PostCategory(request *http.Request) *http.Response {
	return controller.route().Create(request)
}

func (controller *CategoriesController) route() CategoriesRoute {
	return CategoriesRoute{Parts: MailboxParts{Access: controller.accessHelper, Categories: controller.categoryMappingService}}
}
