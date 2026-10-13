import "net/http"

// @RequestRoute("/mcp/config")
// @SignedIn
type McpConfigController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	mcpAccessService *McpAccessService
}

// @GetRoute
func (controller *McpConfigController) GetConfig(request *http.Request) *http.Response {
	return controller.route().Config(request)
}

func (controller *McpConfigController) route() McpSettingsRoute {
	return McpSettingsRoute{Access: controller.accessHelper, Mcp: controller.mcpAccessService}
}

// @PutRoute
func (controller *McpConfigController) PutConfig(request *http.Request) *http.Response {
	return controller.route().UpdateConfig(request)
}
