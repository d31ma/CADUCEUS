import (
	"cmp"

	"mesail.dev/packages/oidc"
)

// @Service
type DomainRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	domains *DomainService
	// @Needs
	memberships *MembershipService
}

func (service *DomainRouteService) List(authorization string) ([]Domain, error) {
	return service.listUnless(service.operator(authorization))
}

func (service *DomainRouteService) Get(authorization string, domainId string) (Domain, error) {
	caller, refused := service.callers.Caller(authorization)
	return service.getUnless(cmp.Or(refused, service.callers.ServiceFor(caller, domainId)), domainId)
}

func (service *DomainRouteService) Add(authorization string, request CreateDomainRequest) (Domain, error) {
	return service.addUnless(service.operator(authorization), request)
}

func (service *DomainRouteService) Verify(authorization string, domainId string) (Domain, error) {
	return service.verifyUnless(service.operator(authorization), domainId)
}

func (service *DomainRouteService) Remove(authorization string, domainId string) (Domain, error) {
	return service.removeUnless(service.operator(authorization), domainId)
}

func (service *DomainRouteService) AddMember(authorization string, domainId string, request AddMemberRequest) (AddedMember, error) {
	caller, refused := service.callers.Caller(authorization)
	return service.addMemberUnless(cmp.Or(refused, service.callers.Operator(caller)), caller, domainId, request)
}

func (service *DomainRouteService) addMemberUnless(refused error, caller oidc.Caller, domainId string, request AddMemberRequest) (AddedMember, error) {
	if refused != nil {
		return AddedMember{}, refused
	}
	return service.memberships.Add(caller, domainId, request)
}

func (service *DomainRouteService) operator(authorization string) error {
	caller, refused := service.callers.Caller(authorization)
	return cmp.Or(refused, service.callers.Operator(caller))
}

func (service *DomainRouteService) getUnless(refused error, domainId string) (Domain, error) {
	if refused != nil {
		return Domain{}, refused
	}
	return service.domains.Get(domainId)
}

func (service *DomainRouteService) listUnless(refused error) ([]Domain, error) {
	if refused != nil {
		return nil, refused
	}
	return service.domains.List()
}

func (service *DomainRouteService) addUnless(refused error, request CreateDomainRequest) (Domain, error) {
	if refused != nil {
		return Domain{}, refused
	}
	return service.domains.Add(request)
}

func (service *DomainRouteService) verifyUnless(refused error, domainId string) (Domain, error) {
	if refused != nil {
		return Domain{}, refused
	}
	return service.domains.Verify(domainId)
}

func (service *DomainRouteService) removeUnless(refused error, domainId string) (Domain, error) {
	if refused != nil {
		return Domain{}, refused
	}
	return service.domains.Remove(domainId)
}
