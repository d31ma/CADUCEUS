package login

import (
	"testing"

	"mesail.dev/packages/oidc"
)

func TestPasskeySignInOpensTheDefaultDomain(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.logins().PasskeySignIn("challenge-key", "{}", "")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "example.org")
	same(t, "content", world.lastSealed(t).Domain, "example.org")
}

func TestPasskeySignInOpensAChosenDomain(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.logins().PasskeySignIn("challenge-key", "{}", "example.com")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "example.com")
}

func TestPasskeySignInClosesTheSessionForADomainThePersonIsNotIn(t *testing.T) {
	world := newLoginWorld(t)
	_, err := world.logins().PasskeySignIn("challenge-key", "{}", "example.net")
	isError(t, err, NotAMember{})
	same(t, "signed out", world.realm.signedOut, []string{"refresh-1"})
}

func TestPasskeySignInPassesARealmRefusalThrough(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.signInErr = oidc.SignInRefused{}
	_, err := world.logins().PasskeySignIn("challenge-key", "{}", "")
	isError(t, err, oidc.SignInRefused{})
	same(t, "events", world.events, []string{"passkey challenge-key"})
}
