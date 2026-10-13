import (
	"errors"
	"net/http"
	"regexp"
	"strings"
)

// RouteAccess asks the authorization boundary whether a request may act, and
// answers the caller the route works for. Tachyon's @SignedIn proves only
// that the identity provider recognised the token; the action and its target
// are the boundary's decision, and a missing or failing answer refuses.
type RouteAccess struct {
	Auth AuthClient
}

// Actions the authorization boundary understands.
func (RouteAccess) MailboxRead() string      { return "mailbox:read" }
func (RouteAccess) MailboxWrite() string     { return "mailbox:write" }
func (RouteAccess) MailboxSend() string      { return "mailbox:send" }
func (RouteAccess) DomainRead() string       { return "domain:read" }
func (RouteAccess) DomainAdmin() string      { return "domain:admin" }
func (RouteAccess) CredentialManage() string { return "credential:manage" }

// Signed answers the caller of any live session.
func (access RouteAccess) Signed(request *http.Request) (RouteCaller, *http.Response) {
	token := access.Bearer(request)
	if token == "" {
		return RouteCaller{}, Answer{}.Unauthorized("Authentication required")
	}
	principal, err := access.Auth.Authenticate(token)
	if err != nil {
		return RouteCaller{}, access.Failure(err)
	}
	return access.caller(principal, "", token), nil
}

// Allowed answers the caller when the boundary permits action. domain or
// mailbox names the target when the request names one; otherwise the action
// applies to what the caller owns, which is the least authority available.
func (access RouteAccess) Allowed(request *http.Request, action string, domain string, mailbox string) (RouteCaller, *http.Response) {
	token := access.Bearer(request)
	if token == "" {
		return RouteCaller{}, Answer{}.Unauthorized("Authentication required")
	}
	if domain != "" && access.domainPattern().MatchString(domain) {
		return access.decide(token, action, map[string]string{"domain": domain})
	}
	if mailbox != "" && access.addressPattern().MatchString(mailbox) {
		return access.decide(token, action, map[string]string{"mailbox": mailbox})
	}
	principal, err := access.Auth.Authenticate(token)
	if err != nil {
		return RouteCaller{}, access.Failure(err)
	}
	if strings.HasPrefix(action, "domain:") {
		return access.decide(token, action, map[string]string{"domain": principal.Domain})
	}
	return access.decide(token, action, map[string]string{"mailbox": principal.Address})
}

func (access RouteAccess) decide(token string, action string, target map[string]string) (RouteCaller, *http.Response) {
	decision, err := access.Auth.Authorize(token, action, target)
	if err != nil {
		return RouteCaller{}, access.Failure(err)
	}
	if !decision.Allowed {
		return RouteCaller{}, Answer{}.Forbidden("Access denied")
	}
	return access.caller(decision.Principal, decision.Domain, token), nil
}

// Storage ownership follows the domain the boundary authorized, not the
// caller's own address.
func (RouteAccess) caller(principal Principal, domain string, token string) RouteCaller {
	if domain == "" {
		domain = principal.Domain
	}
	return RouteCaller{Principal: principal, Token: token, Email: principal.Address, Domain: domain, Domains: []string{domain}}
}

// Bearer answers the bearer token of a request.
func (RouteAccess) Bearer(request *http.Request) string {
	value := strings.TrimSpace(request.Header.Get("Authorization"))
	if len(value) < 7 || !strings.EqualFold(value[:7], "bearer ") {
		return ""
	}
	return strings.TrimSpace(value[7:])
}

// Failure answers a boundary failure with the status earlier versions used.
// An error that did not come from the boundary is the server's own failure:
// it answers 500, not a misleading "Authentication is unavailable".
func (RouteAccess) Failure(err error) *http.Response {
	var failure *BoundaryError
	if !errors.As(err, &failure) {
		return Answer{}.Json(http.StatusInternalServerError, map[string]any{"error": "The server could not complete the request"})
	}
	switch failure.Code {
	case "EBADREQUEST", "EUNSUPPORTED":
		return Answer{}.BadRequest(failure.Message)
	case "EUNAUTHORIZED":
		return Answer{}.Unauthorized("Authentication required")
	case "EPOLICY":
		return Answer{}.Forbidden(failure.Message)
	case "EFORBIDDEN":
		return Answer{}.Forbidden("Access denied")
	case "ECONFLICT":
		return Answer{}.Conflict(failure.Message)
	default:
		return Answer{}.Unavailable("Authentication is unavailable")
	}
}

func (RouteAccess) domainPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
}

func (RouteAccess) addressPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)^[a-z0-9._-]+@[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
}
