package oidc

import "time"

type TokenVerifier interface {
	Verify(rawToken string, now time.Time) (Claims, error)
}
