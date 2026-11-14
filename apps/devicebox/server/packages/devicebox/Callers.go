package devicebox

import "mesail.dev/packages/oidc"

type Callers interface {
	Member(authorization string) (oidc.Caller, error)
	Service(authorization string) (oidc.Caller, error)
}
