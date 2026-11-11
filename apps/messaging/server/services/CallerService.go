import (
	"cmp"
	"slices"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

// @Service
type CallerService struct {
}

func (service *CallerService) ChatCaller(authorization string) (oidc.Caller, error) {
	caller, failure := oidc.Configured().Identify(authorization)
	return caller, cmp.Or(service.refusal(failure), service.chatRefusal(caller.Roles))
}

func (service *CallerService) ServiceCaller(authorization string) (oidc.Caller, error) {
	caller, failure := oidc.Configured().Identify(authorization)
	return caller, cmp.Or(service.refusal(failure), service.serviceRefusal(caller.Roles))
}

func (service *CallerService) refusal(failure error) error {
	if failure != nil {
		return &Unauthenticated{Message: "the access token was refused"}
	}
	return nil
}

func (service *CallerService) chatRefusal(roles []string) error {
	if slices.ContainsFunc(roles, service.chatRole) {
		return nil
	}
	return &core.Forbidden{Message: "only members of a domain use its conversations"}
}

func (service *CallerService) serviceRefusal(roles []string) error {
	if slices.Contains(roles, service.serviceRole()) {
		return nil
	}
	return &core.Forbidden{Message: "only a caller with the Service role may use this route"}
}

func (service *CallerService) chatRole(role string) bool {
	return role == service.memberRole() || role == service.administratorRole()
}

func (service *CallerService) memberRole() string {
	return "Member"
}

func (service *CallerService) administratorRole() string {
	return "DomainAdmin"
}

func (service *CallerService) serviceRole() string {
	return "Service"
}
