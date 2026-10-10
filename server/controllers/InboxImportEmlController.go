import "net/http"

// @RequestRoute("/inbox/import/eml")
// @SignedIn
type InboxImportEmlController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	categoryMappingService *CategoryMappingService
	// @Needs
	emlImportService *EmlImportService
}

// @PostRoute
func (controller *InboxImportEmlController) PostImport(request *http.Request) *http.Response {
	return ImportEmlRoute{Access: controller.accessHelper, Categories: controller.categoryMappingService, Imports: controller.emlImportService}.Import(request)
}
