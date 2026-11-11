package oidc

import "time"

type RefusingVerifier struct{}

func (RefusingVerifier) Verify(rawToken string, now time.Time) (Claims, error) {
	return Claims{}, NotConfigured{}
}
