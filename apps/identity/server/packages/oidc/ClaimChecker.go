package oidc

import "time"

type ClaimChecker interface {
	Check(claims Claims, now time.Time) error
}
