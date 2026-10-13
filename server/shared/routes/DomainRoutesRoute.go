import "net/http"

// DomainRoutesRoute answers /domains/{domain}/routes and
// /domains/{domain}/routes/{id}: the inbound routing rules of one hosted
// domain.
type DomainRoutesRoute struct {
	Access  *AccessHelper
	Domains *DomainsHelper
}

// List answers the routing rules of a domain the caller belongs to.
func (route DomainRoutesRoute) List(domain string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if domain == "" {
		return Answer{}.NotFound("Domain not found")
	}
	if !(PreferenceChoices{}).Includes(caller.Domains, domain) {
		return Answer{}.Forbidden("Access denied")
	}
	_, config, err := route.Domains.FindEntry(domain)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if config == nil {
		return Answer{}.NotFound("Domain not found")
	}
	return Answer{}.Ok(config["routes"])
}

// Remove deletes one routing rule of a domain the caller administers.
func (route DomainRoutesRoute) Remove(target DomainRule, request *http.Request) *http.Response {
	if _, refusal := (RequestBody{}).Value(request); refusal != nil {
		return refusal
	}
	documentId, config, refusal := route.entry(target, request)
	if refusal != nil {
		return refusal
	}
	return route.store(documentId, route.without(config, target.Id), map[string]any{"deleted": target.Id})
}

// Replace stores one routing rule of a domain the caller administers under
// its id, replacing the rule with that id.
func (route DomainRoutesRoute) Replace(target DomainRule, request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	if refusal := route.authorized(target, request); refusal != nil {
		return refusal
	}
	if !(RecordValues{}).Truthy(body) {
		return Answer{}.BadRequest("Missing body")
	}
	rule, _ := RecordValues{}.Object(body)
	if problem := (RouteRules{}).InvalidAction(rule["action"]); problem != "" {
		return Answer{}.BadRequest(problem)
	}
	documentId, config, refusal := route.found(target.Domain)
	if refusal != nil {
		return refusal
	}
	replacement := StoredRecords{}.Without(rule)
	replacement["id"] = target.Id
	return route.store(documentId, append(route.without(config, target.Id), replacement), map[string]any{"updated": target.Id})
}

// entry authorizes a change to target and answers its domain's document.
func (route DomainRoutesRoute) entry(target DomainRule, request *http.Request) (string, map[string]any, *http.Response) {
	if refusal := route.authorized(target, request); refusal != nil {
		return "", nil, refusal
	}
	return route.found(target.Domain)
}

// authorized answers why the caller may not change target, or nil.
func (route DomainRoutesRoute) authorized(target DomainRule, request *http.Request) *http.Response {
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.DomainAdmin(), target.Domain, "")
	if refusal != nil {
		return refusal
	}
	if target.Domain == "" || target.Id == "" {
		return Answer{}.BadRequest("domain and route id required")
	}
	if !(PreferenceChoices{}).Includes(caller.Domains, target.Domain) {
		return Answer{}.Forbidden("Access denied")
	}
	return nil
}

func (route DomainRoutesRoute) found(domain string) (string, map[string]any, *http.Response) {
	documentId, config, err := route.Domains.FindEntry(domain)
	if err != nil {
		return "", nil, RouteAccess{}.Failure(err)
	}
	if documentId == "" || config == nil {
		return "", nil, Answer{}.NotFound("Domain not found")
	}
	return documentId, config, nil
}

// without answers the domain's rules except the one with id.
func (DomainRoutesRoute) without(config map[string]any, id string) []any {
	rules, _ := RecordValues{}.List(config["routes"])
	kept := []any{}
	for _, item := range rules {
		if rule, _ := (RecordValues{}).Object(item); rule["id"] != id {
			kept = append(kept, item)
		}
	}
	return kept
}

func (route DomainRoutesRoute) store(documentId string, rules []any, answer map[string]any) *http.Response {
	if err := route.Domains.UpdateRoutes(documentId, rules); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(answer)
}
