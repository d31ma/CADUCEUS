import (
	"slices"
	"strings"

	"mesail.dev/packages/oidc"
)

// @Service
type CallerService struct {
}

func (service *CallerService) Caller(authorization string) (oidc.Caller, error) {
	caller, failure := oidc.Configured().Identify(authorization)
	return caller, service.refusal(failure)
}

func (service *CallerService) Operator(caller oidc.Caller) error {
	return service.operatorRefusal(caller.DomainId == service.operatorRealm())
}

func (service *CallerService) ServiceFor(caller oidc.Caller, domainId string) error {
	return service.notFoundUnless(slices.Contains([]string{strings.ToLower(domainId), service.operatorRealm()}, caller.DomainId))
}

func (service *CallerService) notFoundUnless(allowed bool) error {
	if allowed {
		return nil
	}
	return &NotFound{Message: "not found"}
}

func (service *CallerService) refusal(failure error) error {
	if failure != nil {
		return &Unauthenticated{Message: "the access token was refused"}
	}
	return nil
}

func (service *CallerService) operatorRefusal(fromOperatorRealm bool) error {
	if fromOperatorRealm {
		return nil
	}
	return &Forbidden{Message: "domains are managed from the operator realm only"}
}

func (service *CallerService) operatorRealm() string {
	return "mesail-operators"
}
