import "net/http"

// @RequestRoute("/mcp/token")
// @SignedIn
type McpTokenController struct {
	// @Needs
	mcpAccessService *McpAccessService
	// @Needs
	accessHelper *AccessHelper
}

// @DeleteRoute
func (controller *McpTokenController) DeleteToken(request *http.Request) *http.Response {
	return McpSettingsRoute{Access: controller.accessHelper, Mcp: controller.mcpAccessService}.Revoke(request)
}

// @PostRoute
func (controller *McpTokenController) PostToken(request *http.Request) *http.Response {
	return McpSettingsRoute{Access: controller.accessHelper, Mcp: controller.mcpAccessService}.Rotate(request)
}
