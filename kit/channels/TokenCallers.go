package channels

import (
	"slices"

	"mesail.dev/packages/oidc"
)

type TokenCallers struct{}

func (callers TokenCallers) Caller(authorization string) (oidc.Caller, error) {
	caller, err := oidc.Configured().Identify(authorization)
	if err != nil {
		return oidc.Caller{}, &Unauthenticated{Message: "the access token was refused"}
	}
	return caller, callers.chatRefusal(caller.Roles)
}

func (callers TokenCallers) chatRefusal(roles []string) error {
	if slices.Contains(roles, callers.member()) || slices.Contains(roles, Rules{}.administrator()) {
		return nil
	}
	return &Forbidden{Message: "only members of a domain use its channels"}
}

func (TokenCallers) member() string {
	return "Member"
}
