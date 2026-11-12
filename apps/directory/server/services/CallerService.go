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
	return service.operatorRefusal(slices.Contains(caller.Roles, service.operatorRole()))
}

func (service *CallerService) ServiceFor(caller oidc.Caller, domainId string) error {
	allowed := []string{oidc.InstallWide{}.DomainId(), strings.ToLower(domainId)}
	return service.notFoundUnless(service.permitted(caller, allowed))
}

func (service *CallerService) permitted(caller oidc.Caller, allowed []string) bool {
	return slices.Contains(allowed, caller.DomainId) || slices.Contains(caller.Roles, service.operatorRole())
}

func (service *CallerService) InstallWide(caller oidc.Caller) error {
	return service.notFoundUnless(oidc.InstallWide{}.Matches(caller.DomainId))
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

func (service *CallerService) operatorRefusal(operator bool) error {
	if operator {
		return nil
	}
	return &Forbidden{Message: "only an Operator may do this"}
}

func (service *CallerService) operatorRole() string {
	return "Operator"
}
