package realtime

import (
	"time"

	"mesail.dev/packages/oidc"
)

type OidcTokenChecker struct {
	Verifier oidc.TokenVerifier
}

func (checker OidcTokenChecker) Grant(token string, now time.Time) (Grant, error) {
	return Grant{}, Unauthenticated{}
}
