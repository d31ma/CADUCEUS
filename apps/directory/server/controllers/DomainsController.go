import (
	"cmp"

	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/directory/domains")
// @Allow(Operator)
type DomainsController struct {
	// @Needs
	callers *CallerService
	// @Needs
	domains *DomainService
	// @Needs
	memberships *MembershipService
}

// @GetRoute
func (controller *DomainsController) GetDomains(
	// @FromHeader("authorization")
	authorization string,
) ([]Domain, error) {
	return controller.listUnless(controller.operator(authorization))
}

// @GetRoute("/{domain_id}")
// @Allow(Operator, Service)
func (controller *DomainsController) GetDomain(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("domain_id")
	domainId string,
) (Domain, error) {
	caller, refused := controller.callers.Caller(authorization)
	return controller.getUnless(cmp.Or(refused, controller.callers.ServiceFor(caller, domainId)), domainId)
}

// @PostRoute
// @ResponseStatus(201)
func (controller *DomainsController) PostDomain(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request CreateDomainRequest,
) (Domain, error) {
	return controller.addUnless(controller.operator(authorization), request)
}

// @PostRoute("/{domain_id}/verify")
func (controller *DomainsController) PostVerify(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("domain_id")
	domainId string,
) (Domain, error) {
	return controller.verifyUnless(controller.operator(authorization), domainId)
}

// @DeleteRoute("/{domain_id}")
// @ResponseStatus(202)
func (controller *DomainsController) DeleteDomain(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("domain_id")
	domainId string,
) (Domain, error) {
	return controller.removeUnless(controller.operator(authorization), domainId)
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
	caller, refused := controller.callers.Caller(authorization)
	return controller.addMemberUnless(cmp.Or(refused, controller.callers.Operator(caller)), caller, domainId, request)
}

func (controller *DomainsController) addMemberUnless(refused error, caller oidc.Caller, domainId string, request AddMemberRequest) (AddedMember, error) {
	if refused != nil {
		return AddedMember{}, refused
	}
	return controller.memberships.Add(caller, domainId, request)
}

func (controller *DomainsController) operator(authorization string) error {
	caller, refused := controller.callers.Caller(authorization)
	return cmp.Or(refused, controller.callers.Operator(caller))
}

func (controller *DomainsController) getUnless(refused error, domainId string) (Domain, error) {
	if refused != nil {
		return Domain{}, refused
	}
	return controller.domains.Get(domainId)
}

func (controller *DomainsController) listUnless(refused error) ([]Domain, error) {
	if refused != nil {
		return nil, refused
	}
	return controller.domains.List()
}

func (controller *DomainsController) addUnless(refused error, request CreateDomainRequest) (Domain, error) {
	if refused != nil {
		return Domain{}, refused
	}
	return controller.domains.Add(request)
}

func (controller *DomainsController) verifyUnless(refused error, domainId string) (Domain, error) {
	if refused != nil {
		return Domain{}, refused
	}
	return controller.domains.Verify(domainId)
}

func (controller *DomainsController) removeUnless(refused error, domainId string) (Domain, error) {
	if refused != nil {
		return Domain{}, refused
	}
	return controller.domains.Remove(domainId)
}
