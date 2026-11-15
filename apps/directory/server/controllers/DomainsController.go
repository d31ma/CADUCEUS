// @RequestRoute("/api/v1/directory/domains")
// @Allow(Operator)
type DomainsController struct {
	// @Needs
	domains *DomainRouteService
}

// @GetRoute("/{domain_id}")
// @Allow(Operator, Service)
func (controller *DomainsController) GetDomain(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("domain_id")
	domainId string,
) (Domain, error) {
	return controller.domains.Get(authorization, domainId)
}

// @GetRoute
func (controller *DomainsController) GetDomains(
	// @FromHeader("authorization")
	authorization string,
) ([]Domain, error) {
	return controller.domains.List(authorization)
}

// @PostRoute
// @ResponseStatus(201)
func (controller *DomainsController) PostDomain(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request CreateDomainRequest,
) (Domain, error) {
	return controller.domains.Add(authorization, request)
}

// @PostRoute("/{domain_id}/members")
// @ResponseStatus(201)
func (controller *DomainsController) PostMember(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("domain_id")
	domainId string,
	// @FromBody
	request AddMemberRequest,
) (AddedMember, error) {
	return controller.domains.AddMember(authorization, domainId, request)
}

// @PostRoute("/{domain_id}/verify")
func (controller *DomainsController) PostVerify(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("domain_id")
	domainId string,
) (Domain, error) {
	return controller.domains.Verify(authorization, domainId)
}

// @DeleteRoute("/{domain_id}")
// @ResponseStatus(202)
func (controller *DomainsController) DeleteDomain(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("domain_id")
	domainId string,
) (Domain, error) {
	return controller.domains.Remove(authorization, domainId)
}
