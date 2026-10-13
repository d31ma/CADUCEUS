import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// McpRoute answers POST /mcp, the stateless MCP endpoint. The caller signs
// in with a separately issued, hash-only MCP capability rather than a
// browser session, before the protocol is touched.
type McpRoute struct {
	Mcp         *McpAccessService
	RateLimit   *RateLimitService
	Audit       *McpAuditEventsHelper
	Fylo        *FyloHelper
	Mailbox     *McpMailboxService
}

// Serve checks origin, content type, size, rate limits and the capability,
// then answers the JSON-RPC exchange.
func (route McpRoute) Serve(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	if refusal := route.precheck(request, body); refusal != nil {
		return refusal
	}
	principal, refusal := route.principal(request)
	if refusal != nil {
		return refusal
	}
	response := McpProtocol{Call: func(tool McpTool, input map[string]any) McpJsonObject {
		return route.execute(*principal, tool, input)
	}}.Answer(body, request.Header)
	response.Header.Set("Cache-Control", "no-store")
	response.Header.Set("X-Content-Type-Options", "nosniff")
	return response
}

func (route McpRoute) precheck(request *http.Request, body any) *http.Response {
	if origin := request.Header.Get("Origin"); origin != "" {
		host := request.Host
		if host == "" {
			host = request.Header.Get("Host")
		}
		parsed, err := url.Parse(origin)
		if err != nil || host == "" || parsed.Host == "" || !strings.EqualFold(parsed.Host, host) {
			return route.json(http.StatusForbidden, map[string]any{"error": "Cross-origin MCP requests are not allowed"})
		}
	}
	if !regexp.MustCompile(`(?i)^application/json(?:\s*;|$)`).MatchString(request.Header.Get("Content-Type")) {
		return route.json(http.StatusUnsupportedMediaType, map[string]any{"error": "Content-Type must be application/json"})
	}
	if len(TextValue{}.Json(body)) > 1024*1024 {
		return route.json(http.StatusRequestEntityTooLarge, map[string]any{"error": "MCP request is too large"})
	}
	return route.limited("mcp-auth", route.address(request))
}

func (route McpRoute) principal(request *http.Request) (*McpPrincipal, *http.Response) {
	match := regexp.MustCompile(`(?i)^Bearer ([^\s]{1,420})$`).FindStringSubmatch(request.Header.Get("Authorization"))
	var principal *McpPrincipal
	if match != nil {
		found, err := route.Mcp.Authenticate(match[1])
		if err != nil {
			return nil, RouteAccess{}.Failure(err)
		}
		principal = found
	}
	if principal == nil {
		response := route.json(http.StatusUnauthorized, map[string]any{"error": "Invalid or disabled MCP capability"})
		response.Header.Set("WWW-Authenticate", `Bearer realm="caduceus-mcp"`)
		return nil, response
	}
	return principal, route.limited("mcp-request", principal.TokenId)
}

func (route McpRoute) limited(scope string, key string) *http.Response {
	limit, err := route.RateLimit.Check([]string{scope, key}, 600, 60_000)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if !limit.Allowed {
		return route.json(http.StatusTooManyRequests, map[string]any{"error": "Too many MCP requests", "retryAfterSeconds": limit.RetryAfterSeconds})
	}
	return nil
}

// execute runs one tool for the principal and records it in the owner's
// audit trail, whatever the outcome.
func (route McpRoute) execute(principal McpPrincipal, tool McpTool, input map[string]any) McpJsonObject {
	started := time.Now()
	previous := route.Fylo.ActForDomain(principal.PrimaryDomain)
	defer route.Fylo.Restore(previous)
	status, count := "ok", 0
	var answer McpJsonObject
	if !route.permits(principal.AccessLevel, tool.Category) {
		status = "denied"
		answer = McpToolResults{}.Failed(fmt.Sprintf("MCP access level %s does not permit %s tools", principal.AccessLevel, tool.Category))
	} else {
		route.Fylo.ActFor(principal.OwnerEmail, principal.AllowedDomains)
		result, err := route.Mailbox.Run(McpToolCall{Principal: principal, Name: tool.Name, Input: input, Fylo: route.Fylo})
		if err != nil {
			status = "error"
			answer = McpToolResults{}.Failed(err.Error())
		} else {
			count = McpToolResults{}.Count(result)
			answer = McpToolResults{}.Succeeded(result)
		}
	}
	route.Fylo.ActForDomain(principal.PrimaryDomain)
	route.audit(principal, tool, input, status, count, started)
	return answer
}

func (route McpRoute) audit(principal McpPrincipal, tool McpTool, input map[string]any, status string, count int, started time.Time) {
	keys := []string{}
	for key := range input {
		if regexp.MustCompile(`^[a-z][a-zA-Z0-9]{0,63}$`).MatchString(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	err := route.Audit.Put(map[string]any{
		"id":                "mcp-audit-" + CapabilityToken{}.Random(16),
		"ownerEmail":        principal.OwnerEmail,
		"domain":            principal.PrimaryDomain,
		"tokenId":           principal.TokenId,
		"toolName":          tool.Name,
		"category":          tool.Category,
		"status":            status,
		"parameterKeysJson": TextValue{}.Json(keys),
		"resultCount":       count,
		"durationMs":        math.Max(0, math.Round(float64(time.Since(started).Microseconds())/1000)),
		"createdAt":         EpochTime{}.NowText(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "[mcp] audit write failed:", err.Error())
	}
}

func (McpRoute) permits(accessLevel string, category string) bool {
	allowed := map[string][]string{
		"read-only":       {"read"},
		"read-write":      {"read", "write"},
		"read-write-send": {"read", "write", "send"},
	}[accessLevel]
	return (PreferenceChoices{}).Includes(allowed, category)
}

func (McpRoute) address(request *http.Request) string {
	address := Security{}.Trim(strings.Split(request.Header.Get("X-Forwarded-For"), ",")[0])
	if address == "" {
		address = request.Header.Get("X-Real-Ip")
	}
	if address == "" {
		return "unknown"
	}
	return address
}

func (McpRoute) json(status int, body map[string]any) *http.Response {
	response := Answer{}.Json(status, body)
	response.Header.Set("Cache-Control", "no-store")
	return response
}
