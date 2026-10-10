import (
	"errors"
	"net/http"
)

// McpSettingsRoute answers /mcp/config, /mcp/token and /mcp/audit: the
// signed-in owner's MCP configuration, capability and audit trail.
type McpSettingsRoute struct {
	Access *AccessHelper
	Mcp    *McpAccessService
	Audit  *McpAuditEventsHelper
}

// Config answers the caller's MCP configuration.
func (route McpSettingsRoute) Config(request *http.Request) *http.Response {
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxRead(), "", "")
	if refusal != nil {
		return refusal
	}
	return route.userAnswer(route.Mcp.Config(caller.Email))
}

// UpdateConfig changes the caller's MCP configuration.
func (route McpSettingsRoute) UpdateConfig(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	config, err := route.Mcp.UpdateConfig(caller.Email, body)
	var preference *McpPreferenceError
	if errors.As(err, &preference) {
		if preference.Forbidden {
			return Answer{}.Forbidden(preference.Message)
		}
		return Answer{}.BadRequest(preference.Message)
	}
	return route.userAnswer(config, err)
}

// Rotate answers a new capability for the caller.
func (route McpSettingsRoute) Rotate(request *http.Request) *http.Response {
	caller, refusal := route.writer(request)
	if refusal != nil {
		return refusal
	}
	return route.userAnswer(route.Mcp.Rotate(caller.Email))
}

// Revoke revokes the caller's capability.
func (route McpSettingsRoute) Revoke(request *http.Request) *http.Response {
	return route.written(request, "revoked", func(email string) (any, error) { return route.Mcp.Revoke(email) })
}

// AuditTrail answers the caller's newest MCP audit events, without the
// owner or the raw parameter text.
func (route McpSettingsRoute) AuditTrail(request *http.Request) *http.Response {
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxRead(), "", "")
	if refusal != nil {
		return refusal
	}
	events, err := route.Audit.ListForOwner(caller.Email, McpAuditEventsRecords{}.ReturnedEvents())
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	presented := []any{}
	for _, event := range events {
		view := map[string]any{}
		for _, field := range []string{"id", "tokenId", "toolName", "category", "status", "parameterKeys", "resultCount", "durationMs", "createdAt"} {
			if value, present := event[field]; present {
				view[field] = value
			}
		}
		presented = append(presented, view)
	}
	return Answer{}.Ok(presented)
}

// ClearAudit deletes the caller's MCP audit trail.
func (route McpSettingsRoute) ClearAudit(request *http.Request) *http.Response {
	return route.written(request, "cleared", func(email string) (any, error) { return route.Audit.ClearForOwner(email) })
}

// written answers {field: act(caller)} for a caller allowed to change the
// mailbox.
func (route McpSettingsRoute) written(request *http.Request, field string, act func(email string) (any, error)) *http.Response {
	caller, refusal := route.writer(request)
	if refusal != nil {
		return refusal
	}
	value, err := act(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{field: value})
}

func (route McpSettingsRoute) writer(request *http.Request) (RouteCaller, *http.Response) {
	if _, refusal := (RequestBody{}).Value(request); refusal != nil {
		return RouteCaller{}, refusal
	}
	return route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
}

func (McpSettingsRoute) userAnswer(value map[string]any, err error) *http.Response {
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if value == nil {
		return Answer{}.NotFound("User not found")
	}
	return Answer{}.Ok(value)
}
