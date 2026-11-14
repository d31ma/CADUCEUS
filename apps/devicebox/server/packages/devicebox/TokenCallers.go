package devicebox

import (
	"slices"

	"mesail.dev/packages/oidc"
)

type TokenCallers struct{}

func (callers TokenCallers) Member(authorization string) (oidc.Caller, error) {
	caller, err := callers.Service(authorization)
	if err != nil {
		return oidc.Caller{}, err
	}
	return caller, callers.memberRefusal(caller.Roles)
}

func (TokenCallers) Service(authorization string) (oidc.Caller, error) {
	caller, err := oidc.Configured().Identify(authorization)
	if err != nil {
		return oidc.Caller{}, Unauthenticated{}.TokenRefused()
	}
	return caller, nil
}

func (callers TokenCallers) memberRefusal(roles []string) error {
	if slices.ContainsFunc(roles, callers.isMemberRole) {
		return nil
	}
	return Forbidden{}.NotADomainMember()
}

func (TokenCallers) isMemberRole(role string) bool {
	return slices.Contains([]string{"Member", "DomainAdmin"}, role)
}
