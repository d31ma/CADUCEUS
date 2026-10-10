import "net/http"

// @RequestRoute("/mcp")
// @Public
type McpController struct {
	// @Needs
	fyloHelper *FyloHelper
	// @Needs
	mcpAccessService *McpAccessService
	// @Needs
	mcpAuditEventsHelper *McpAuditEventsHelper
	// @Needs
	mcpMailboxService *McpMailboxService
	// @Needs
	rateLimitService *RateLimitService
}

// @PostRoute
func (controller *McpController) PostMessage(request *http.Request) *http.Response {
	route := McpRoute{
		Mcp: controller.mcpAccessService, RateLimit: controller.rateLimitService, Audit: controller.mcpAuditEventsHelper,
		Fylo: controller.fyloHelper, Mailbox: controller.mcpMailboxService,
	}
	return route.Serve(request)
}
