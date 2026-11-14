package oidc

import (
	"errors"
	"slices"
	"time"
)

type MesailClaimRules struct {
	Audience string
}

func (rules MesailClaimRules) Check(claims Claims, now time.Time) error {
	shared := ClaimRules{Audience: rules.Audience}
	return errors.Join(
		shared.audience(claims),
		rules.party(claims),
		shared.expiry(claims, now),
		shared.notBefore(claims, now),
		shared.subject(claims),
		rules.domain(claims),
	)
}

func (rules MesailClaimRules) party(claims Claims) error {
	if !slices.Contains(rules.parties(), claims.AuthorizedParty) {
		return RefusedToken{Reason: "the token was issued to an unknown client"}
	}
	return nil
}

func (MesailClaimRules) domain(claims Claims) error {
	if !(InstallWide{}).Matches(claims.Domain) && !(DomainName{}).Valid(claims.Domain) {
		return RefusedToken{Reason: "the token names no valid domain"}
	}
	return nil
}

func (MesailClaimRules) parties() []string {
	return []string{AuthorizedParties{}.Web(), AuthorizedParties{}.Service()}
}
