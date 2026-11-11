package search

import "mesail.dev/packages/oidc"

type Identifier interface {
	Identify(authorization string) (oidc.Caller, error)
}
