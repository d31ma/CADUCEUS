package realtime

import (
	"errors"
	"slices"
	"time"

	"mesail.dev/packages/oidc"
)

type OidcTokenChecker struct {
	Verifier oidc.TokenVerifier
}

func (checker OidcTokenChecker) Grant(token string, now time.Time) (Grant, error) {
	claims, refused := checker.Verifier.Verify(token, now)
	grant := Grant{DomainId: claims.Domain, MemberId: claims.Subject, ExpiresAt: checker.expiresAt(claims.ExpiresAt, now)}
	return checker.checked(grant, errors.Join(refused, checker.member(claims)))
}

func (checker OidcTokenChecker) member(claims oidc.Claims) error {
	if claims.Subject == "" || !slices.Contains(oidc.RoleMap{}.MesailRoles(claims), checker.memberRole()) {
		return Unauthenticated{}
	}
	return nil
}

func (OidcTokenChecker) checked(grant Grant, failure error) (Grant, error) {
	if failure != nil {
		return Grant{}, Unauthenticated{}
	}
	return grant, nil
}

func (checker OidcTokenChecker) expiresAt(tokenExpiresAt int64, now time.Time) string {
	seconds := min(tokenExpiresAt, now.Unix()+checker.recheckSeconds())
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func (OidcTokenChecker) recheckSeconds() int64 {
	return 60
}

func (OidcTokenChecker) memberRole() string {
	return "Member"
}
