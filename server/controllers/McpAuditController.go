import "net/http"

// @RequestRoute("/mcp/audit")
// @SignedIn
type McpAuditController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	mcpAuditEventsHelper *McpAuditEventsHelper
}

func (controller *McpAuditController) settings() McpSettingsRoute {
	return McpSettingsRoute{Access: controller.accessHelper, Audit: controller.mcpAuditEventsHelper}
}

// @GetRoute
func (controller *McpAuditController) GetAudit(request *http.Request) *http.Response {
	return controller.settings().AuditTrail(request)
}

// @DeleteRoute
func (controller *McpAuditController) DeleteAudit(request *http.Request) *http.Response {
	settings := controller.settings()
	return settings.ClearAudit(request)
}
