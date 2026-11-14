package oidc

import (
	"errors"
	"slices"
	"time"
)

type ClaimRules struct {
	Audience string
}

func (rules ClaimRules) Check(claims Claims, now time.Time) error {
	return errors.Join(
		rules.audience(claims),
		rules.party(claims),
		rules.expiry(claims, now),
		rules.notBefore(claims, now),
		rules.tokenType(claims),
		rules.subject(claims),
	)
}

func (rules ClaimRules) audience(claims Claims) error {
	if !slices.Contains(claims.Audience, rules.Audience) {
		return RefusedToken{Reason: "the token is for another audience"}
	}
	return nil
}

func (rules ClaimRules) party(claims Claims) error {
	if !slices.Contains(rules.parties(), claims.AuthorizedParty) {
		return RefusedToken{Reason: "the token was issued to an unknown client"}
	}
	return nil
}

func (rules ClaimRules) expiry(claims Claims, now time.Time) error {
	if now.Unix() >= claims.ExpiresAt+rules.leewaySeconds() {
		return RefusedToken{Reason: "the token has expired"}
	}
	return nil
}

func (rules ClaimRules) notBefore(claims Claims, now time.Time) error {
	if now.Unix()+rules.leewaySeconds() < claims.NotBefore {
		return RefusedToken{Reason: "the token is not valid yet"}
	}
	return nil
}

func (rules ClaimRules) tokenType(claims Claims) error {
	if claims.TokenType != rules.accessTokenType() {
		return RefusedToken{Reason: "the token is not an access token"}
	}
	return nil
}

func (ClaimRules) subject(claims Claims) error {
	if claims.Subject == "" {
		return RefusedToken{Reason: "the token has no subject"}
	}
	return nil
}

func (ClaimRules) parties() []string {
	return []string{AuthorizedParties{}.Identity()}
}

func (ClaimRules) leewaySeconds() int64 {
	return 30
}

func (ClaimRules) accessTokenType() string {
	return "Bearer"
}
