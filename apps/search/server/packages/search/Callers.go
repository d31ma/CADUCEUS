package search

import (
	"slices"

	"mesail.dev/packages/oidc"
)

type Callers struct {
	Identifier Identifier
}

func (callers Callers) Member(authorization string) (oidc.Caller, error) {
	caller, err := callers.Identifier.Identify(authorization)
	if err != nil {
		return oidc.Caller{}, Unauthenticated{}
	}
	return caller, callers.memberRefusal(caller.Roles)
}

func (callers Callers) memberRefusal(roles []string) error {
	if slices.Contains(roles, callers.member()) {
		return nil
	}
	return Forbidden{Message: "only members of a domain search its messages"}
}

func (Callers) member() string {
	return "Member"
}
