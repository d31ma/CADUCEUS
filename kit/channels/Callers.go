package channels

import "mesail.dev/packages/oidc"

type Callers interface {
	Caller(authorization string) (oidc.Caller, error)
}
