import (
	"net/http"
	"strings"
)

// UsersRoute answers /users and /users/{id}: the mailbox records a domain
// administrator manages.
type UsersRoute struct {
	Access *AccessHelper
	Users  *UsersHelper
}

// Create stores a new user of domains the caller administers.
func (route UsersRoute) Create(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.DomainAdmin(), route.firstDomain(body), "")
	if refusal != nil {
		return refusal
	}
	user, problem := route.checked(caller, body)
	if problem != nil {
		return problem
	}
	email := user["email"].(string)
	for _, address := range append([]string{email}, user["aliases"].([]string)...) {
		existingId, existing, err := route.Users.FindByEmail(address)
		if err != nil {
			return RouteAccess{}.Failure(err)
		}
		if existingId != "" && existing["email"] != email {
			return Answer{}.BadRequest("address already belongs to another user: " + address)
		}
	}
	if _, err := route.Users.Put(user); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"email": email})
}

// checked answers the normalized user the body describes, or the answer that
// refuses it.
func (route UsersRoute) checked(caller RouteCaller, body map[string]any) (map[string]any, *http.Response) {
	rawDomains, _ := RecordValues{}.List(body["domains"])
	switch {
	case !(RecordValues{}).Truthy(body["email"]):
		return nil, Answer{}.BadRequest("email required")
	case len(rawDomains) == 0:
		return nil, Answer{}.BadRequest("domains required")
	case !(RecordValues{}).Truthy(body["role"]):
		return nil, Answer{}.BadRequest("role required")
	case body["role"] != "admin" && body["role"] != "viewer":
		return nil, Answer{}.BadRequest("role invalid")
	}
	email := Security{}.NormalizeEmailAddress(BodyText{}.String(body["email"]))
	if email == "" {
		return nil, Answer{}.BadRequest("email invalid")
	}
	rawAliases, _ := RecordValues{}.List(body["aliases"])
	aliases := []string{}
	for _, raw := range rawAliases {
		alias := Security{}.NormalizeEmailAddress(BodyText{}.String(raw))
		if alias == "" {
			return nil, Answer{}.BadRequest("aliases invalid")
		}
		aliases = append(aliases, alias)
	}
	domains := []string{}
	for _, raw := range rawDomains {
		domain := Security{}.NormalizeDomain(BodyText{}.String(raw))
		if domain == "" {
			return nil, Answer{}.BadRequest("domains invalid")
		}
		domains = append(domains, domain)
	}
	if refusal := route.refusedAddresses(caller, email, aliases, domains); refusal != nil {
		return nil, refusal
	}
	return map[string]any{"email": email, "aliases": aliases, "domains": domains, "role": body["role"]}, nil
}

func (UsersRoute) refusedAddresses(caller RouteCaller, email string, aliases []string, domains []string) *http.Response {
	choices := PreferenceChoices{}
	if choices.Includes(aliases, email) {
		return Answer{}.BadRequest("aliases must not include primary email")
	}
	for _, alias := range aliases {
		if !choices.Includes(domains, strings.Split(alias, "@")[1]) {
			return Answer{}.BadRequest("aliases must belong to user domains")
		}
	}
	for _, domain := range domains {
		if !(Security{}).HasDomainClaim(caller.Domains, domain) {
			return Answer{}.Forbidden("Domain access denied")
		}
	}
	return nil
}

// List answers every user to a domain administrator.
func (route UsersRoute) List(request *http.Request) *http.Response {
	if _, refusal := route.Access.Allowed(request, (RouteAccess{}).DomainAdmin(), "", ""); refusal != nil {
		return refusal
	}
	users, err := route.Users.List()
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(users)
}

// Delete deletes the user with the primary address or alias id.
func (route UsersRoute) Delete(id string, request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	if _, refusal := route.Access.Allowed(request, (RouteAccess{}).DomainAdmin(), route.firstDomain(body), ""); refusal != nil {
		return refusal
	}
	if id == "" {
		return Answer{}.BadRequest("email required")
	}
	documentId, _, err := route.Users.FindByEmail(id)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if documentId == "" {
		return Answer{}.NotFound("User not found")
	}
	if err := route.Users.Delete(documentId); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"deleted": id})
}

// firstDomain answers the first domain the body names, which the boundary
// authorizes the request against.
func (UsersRoute) firstDomain(body map[string]any) string {
	domains, _ := RecordValues{}.List(body["domains"])
	if len(domains) == 0 {
		return ""
	}
	return RecordValues{}.Text(domains[0])
}
