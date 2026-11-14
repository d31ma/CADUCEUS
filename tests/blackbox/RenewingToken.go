//go:build blackbox

package blackbox

import (
	"testing"
	"time"
)

type RenewingToken struct {
	stack    Stack
	name     string
	token    string
	signedAt time.Time
}

func NewRenewingToken(t *testing.T, stack Stack, name string) *RenewingToken {
	t.Helper()
	renewing := &RenewingToken{stack: stack, name: name}
	renewing.signIn(t)
	return renewing
}

func (renewing *RenewingToken) Get(t *testing.T) string {
	t.Helper()
	if time.Since(renewing.signedAt) > 45*time.Second {
		renewing.signIn(t)
	}
	return renewing.token
}

func (renewing *RenewingToken) signIn(t *testing.T) {
	t.Helper()
	renewing.token = fixture(t, renewing.stack, renewing.name).AccessToken
	renewing.signedAt = time.Now()
}
