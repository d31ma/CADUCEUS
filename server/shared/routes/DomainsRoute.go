import "net/http"

// DomainsRoute answers /domains: the hosted domains the caller administers.
type DomainsRoute struct {
	Access  *AccessHelper
	Domains *DomainsHelper
}

// List answers the caller's domain configurations.
func (route DomainsRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	configs, err := route.Domains.List(caller.Domains)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(configs)
}

// Create stores a new domain configuration for a domain the caller
// administers.
func (route DomainsRoute) Create(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	requested, _ := body["domain"].(string)
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.DomainAdmin(), requested, "")
	if refusal != nil {
		return refusal
	}
	return route.store(caller, body, requested)
}

func (route DomainsRoute) store(caller RouteCaller, body map[string]any, requested string) *http.Response {
	if requested == "" {
		return Answer{}.BadRequest("domain required")
	}
	domain := Security{}.NormalizeDomain(requested)
	if domain == "" {
		return Answer{}.BadRequest("domain invalid")
	}
	if !(Security{}).HasDomainClaim(caller.Domains, domain) {
		return Answer{}.Forbidden("Domain access denied")
	}
	routes := body["routes"]
	if routes == nil {
		routes = []any{}
	}
	if problem := (RouteRules{}).Invalid(routes); problem != "" {
		return Answer{}.BadRequest(problem)
	}
	inbound, _ := body["inboundEnabled"].(bool)
	if _, err := route.Domains.Put(map[string]any{"domain": domain, "routes": routes, "inboundEnabled": inbound}); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"domain": domain})
}
