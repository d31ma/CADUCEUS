package login

import (
	"testing"

	"mesail.dev/packages/oidc"
)

func signInAs(t *testing.T, login string) (*loginWorld, Grant) {
	t.Helper()
	world := newLoginWorld(t)
	grant, err := world.logins().SignIn(login, "right-password", "")
	failOn(t, err)
	return world, grant
}

func refusedLogin(t *testing.T, login string) {
	t.Helper()
	world := newLoginWorld(t)
	_, err := world.logins().SignIn(login, "right-password", "")
	isError(t, err, oidc.SignInRefused{})
	same(t, "events", world.events, []string(nil))
}

func TestSignInAcceptsTheUsername(t *testing.T) {
	world, grant := signInAs(t, "ana")
	same(t, "usernames", world.realm.usernames, []string{"ana"})
	same(t, "person", grant.PersonId, "person-1")
}

func TestSignInLowersTheUsername(t *testing.T) {
	world, grant := signInAs(t, "ANA")
	same(t, "usernames", world.realm.usernames, []string{"ana"})
	same(t, "person", grant.PersonId, "person-1")
}

func TestSignInTrimsTheUsername(t *testing.T) {
	world, grant := signInAs(t, " ana ")
	same(t, "usernames", world.realm.usernames, []string{"ana"})
	same(t, "person", grant.PersonId, "person-1")
}

func TestSignInAcceptsAnAddress(t *testing.T) {
	world, grant := signInAs(t, "ana@example.com")
	same(t, "usernames", world.realm.usernames, []string{"ana"})
	same(t, "person", grant.PersonId, "person-1")
}

func TestSignInRefusesARoleName(t *testing.T) {
	refusedLogin(t, "postmaster")
}

func TestSignInRefusesDoubleDots(t *testing.T) {
	refusedLogin(t, "a..b")
}

func TestSignInRefusesNonAscii(t *testing.T) {
	refusedLogin(t, "anä")
}

func TestSignInRefusesAnEmptyDomain(t *testing.T) {
	refusedLogin(t, "ana@")
}

func TestSignInPassesARealmRefusalThrough(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.signInErr = oidc.AccountLocked{}
	_, err := world.logins().SignIn("ana", "wrong", "")
	isError(t, err, oidc.AccountLocked{})
	same(t, "events", world.events, []string{"sign-in ana"})
}

func TestSignInWithTheUsernameOpensTheDefaultDomain(t *testing.T) {
	_, grant := signInAs(t, "ana")
	same(t, "domain", grant.DomainId, "example.org")
}

func TestSignInWithTheUsernameOpensTheFirstDomainWhenTheDefaultIsGone(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(withDomains(anaPerson(), []string{"example.com", "example.org"}, "gone.example"))
	grant, err := world.logins().SignIn("ana", "right-password", "")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "example.com")
}

func TestSignInWithAnAddressOpensItsDomain(t *testing.T) {
	_, grant := signInAs(t, "ana@example.com")
	same(t, "domain", grant.DomainId, "example.com")
}

func TestSignInRefusesAnAddressOfAnotherDomain(t *testing.T) {
	world := newLoginWorld(t)
	_, err := world.logins().SignIn("ana@other.example", "right-password", "")
	isError(t, err, NotAMember{})
	same(t, "signed out", world.realm.signedOut, []string{"refresh-1"})
}

func TestSignInRefusesAPersonWithoutDomains(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(withDomains(anaPerson(), nil, ""))
	_, err := world.logins().SignIn("ana", "right-password", "")
	isError(t, err, NoDomain{})
	same(t, "signed out", world.realm.signedOut, []string{"refresh-1"})
}

func TestSignInGivesAnOperatorWithoutDomainsTheWholeInstall(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.session.Operator = true
	world.setPerson(withDomains(anaPerson(), nil, ""))
	grant, err := world.logins().SignIn("ana", "right-password", "")
	failOn(t, err)
	claims := signedClaims(t, grant.AccessToken)
	same(t, "domain", grant.DomainId, "*")
	same(t, "address", grant.PrimaryAddress, "")
	same(t, "roles", grant.Roles, []string{"Operator"})
	same(t, "token domain", claims.Domain, "*")
	same(t, "token email", claims.Email, "")
	same(t, "token roles", claims.RealmRoles, []string{"mesail-operator"})
}

func TestSignInRefusesADisabledPerson(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(withState(anaPerson(), "disabled"))
	_, err := world.logins().SignIn("ana", "right-password", "")
	isError(t, err, oidc.SignInRefused{})
	same(t, "signed out", world.realm.signedOut, []string{"refresh-1"})
}

func TestSignInRefusesADeletedPerson(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(withState(anaPerson(), "deleted"))
	_, err := world.logins().SignIn("ana", "right-password", "")
	isError(t, err, oidc.SignInRefused{})
	same(t, "signed out", world.realm.signedOut, []string{"refresh-1"})
}

func TestSignInReturnsADirectoryFailure(t *testing.T) {
	world := newLoginWorld(t)
	world.people.err = oidc.SessionUnavailable{}
	_, err := world.logins().SignIn("ana", "right-password", "")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "signed out", world.realm.signedOut, []string{"refresh-1"})
}

func TestSignInReturnsASignerFailure(t *testing.T) {
	world := newLoginWorld(t)
	world.signer = brokenSigner()
	_, err := world.logins().SignIn("ana", "right-password", "")
	isError(t, err, oidc.NotConfigured{})
	same(t, "sealed", world.realm.sealed, []oidc.SessionContent(nil))
}

func TestSignInReturnsASealFailure(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.sealErr = oidc.SessionUnavailable{}
	_, err := world.logins().SignIn("ana", "right-password", "")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestGrantNamesTheAdminDomain(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.session.MustChangePassword = true
	grant, err := world.logins().SignIn("ana@example.org", "right-password", "")
	failOn(t, err)
	same(t, "grant", grant, Grant{
		PersonId:           "person-1",
		DomainId:           "example.org",
		Username:           "ana",
		DisplayName:        "Ana Example",
		PrimaryAddress:     "ana@example.org",
		Roles:              []string{"Member", "DomainAdmin"},
		Domains:            []DomainChoice{{DomainId: "example.com", Admin: false}, {DomainId: "example.org", Admin: true, Default: true}},
		AccessToken:        grant.AccessToken,
		ExpiresIn:          60,
		SessionKey:         "sealed-key",
		MustChangePassword: true,
	})
	same(t, "sealed", world.lastSealed(t), oidc.SessionContent{Domain: "example.org", RefreshToken: "refresh-1", PersonId: "person-1"})
}

func TestGrantTokenNamesThePersonAndDomain(t *testing.T) {
	_, grant := signInAs(t, "ana@example.org")
	claims := signedClaims(t, grant.AccessToken)
	same(t, "party", claims.AuthorizedParty, "mesail-web")
	same(t, "subject", claims.Subject, "person-1")
	same(t, "session", claims.SessionId, "session-1")
	same(t, "domain", claims.Domain, "example.org")
	same(t, "username", claims.Username, "ana")
	same(t, "name", claims.Name, "Ana Example")
	same(t, "email", claims.Email, "ana@example.org")
	same(t, "roles", claims.RealmRoles, []string{"mesail-member", "mesail-admin"})
}

func TestGrantAddsTheOperatorRoleFromTheRealm(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.session.Operator = true
	grant, err := world.logins().SignIn("ana@example.com", "right-password", "")
	failOn(t, err)
	same(t, "roles", grant.Roles, []string{"Member", "Operator"})
	same(t, "token roles", signedClaims(t, grant.AccessToken).RealmRoles, []string{"mesail-member", "mesail-operator"})
}

func TestGrantFallsBackToTheRealmName(t *testing.T) {
	world := newLoginWorld(t)
	person := anaPerson()
	person.DisplayName = ""
	world.setPerson(person)
	grant, err := world.logins().SignIn("ana", "right-password", "")
	failOn(t, err)
	same(t, "name", grant.DisplayName, "Ana Realm")
	same(t, "token name", signedClaims(t, grant.AccessToken).Name, "Ana Realm")
}

func TestRefreshReadsThePersonBeforeTheRefreshToken(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.logins().Refresh("key-1")
	failOn(t, err)
	same(t, "events", world.events, []string{"open key-1", "person person-1", "refresh refresh-1", "seal"})
	same(t, "domain", grant.DomainId, "example.com")
	same(t, "sealed", world.lastSealed(t), oidc.SessionContent{Domain: "example.com", RefreshToken: "refresh-2", PersonId: "person-1"})
}

func TestRefreshRevokesADisabledPerson(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(withState(anaPerson(), "disabled"))
	_, err := world.logins().Refresh("key-1")
	isError(t, err, AccessRevoked{})
	same(t, "events", world.events, []string{"open key-1", "person person-1"})
}

func TestRefreshRevokesADeletedPerson(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(withState(anaPerson(), "deleted"))
	_, err := world.logins().Refresh("key-1")
	isError(t, err, AccessRevoked{})
	same(t, "events", world.events, []string{"open key-1", "person person-1"})
}

func TestRefreshRefusesARemovedDomain(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.content.Domain = "gone.example"
	_, err := world.logins().Refresh("key-1")
	isError(t, err, NotAMember{})
	same(t, "events", world.events, []string{"open key-1", "person person-1"})
}

func TestRefreshKeepsTheWholeInstallForAnOperator(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.content.Domain = "*"
	world.realm.refreshed.Operator = true
	grant, err := world.logins().Refresh("key-1")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "*")
}

func TestRefreshRefusesTheWholeInstallForAPerson(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.content.Domain = "*"
	_, err := world.logins().Refresh("key-1")
	isError(t, err, NoDomain{})
	same(t, "sealed", world.realm.sealed, []oidc.SessionContent(nil))
}

func TestRefreshRefusesAnotherPerson(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.refreshed.PersonId = "person-2"
	_, err := world.logins().Refresh("key-1")
	isError(t, err, oidc.SignInRefused{})
	same(t, "sealed", world.realm.sealed, []oidc.SessionContent(nil))
}

func TestRefreshPassesAnOpenFailureThrough(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.openErr = oidc.SessionUnavailable{}
	_, err := world.logins().Refresh("key-1")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "events", world.events, []string{"open key-1"})
}

func TestRefreshPassesADirectoryFailureThrough(t *testing.T) {
	world := newLoginWorld(t)
	world.people.err = oidc.SessionUnavailable{}
	_, err := world.logins().Refresh("key-1")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "events", world.events, []string{"open key-1", "person person-1"})
}

func TestRefreshPassesARealmFailureThrough(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.refreshErr = oidc.SignInRefused{}
	_, err := world.logins().Refresh("key-1")
	isError(t, err, oidc.SignInRefused{})
	same(t, "sealed", world.realm.sealed, []oidc.SessionContent(nil))
}

func TestSwitchToNothingOpensTheDefaultDomain(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.logins().Switch("key-1", "")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "example.org")
}

func TestSwitchToNothingOpensTheFirstDomainWhenTheDefaultIsGone(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(withDomains(anaPerson(), []string{"example.com", "example.org"}, "gone.example"))
	grant, err := world.logins().Switch("key-1", "")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "example.com")
}

func TestSwitchToNothingGivesAnOperatorTheWholeInstall(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(withDomains(anaPerson(), nil, ""))
	world.realm.refreshed.Operator = true
	grant, err := world.logins().Switch("key-1", "")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "*")
}

func TestSwitchToNothingRefusesAPersonWithoutDomains(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(withDomains(anaPerson(), nil, ""))
	_, err := world.logins().Switch("key-1", "")
	isError(t, err, NoDomain{})
}

func TestSwitchOpensAMemberDomain(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.logins().Switch("key-1", "example.org")
	failOn(t, err)
	same(t, "domain", grant.DomainId, "example.org")
	same(t, "sealed", world.lastSealed(t), oidc.SessionContent{Domain: "example.org", RefreshToken: "refresh-2", PersonId: "person-1"})
}

func TestSwitchRefusesAnotherDomainWithoutRefreshing(t *testing.T) {
	world := newLoginWorld(t)
	_, err := world.logins().Switch("key-1", "other.example")
	isError(t, err, NotFound{})
	same(t, "events", world.events, []string{"open key-1", "person person-1"})
}

func TestSwitchRevokesADisabledPerson(t *testing.T) {
	world := newLoginWorld(t)
	world.setPerson(withState(anaPerson(), "disabled"))
	_, err := world.logins().Switch("key-1", "example.com")
	isError(t, err, AccessRevoked{})
	same(t, "events", world.events, []string{"open key-1", "person person-1"})
}

func TestSwitchPassesAnOpenFailureThrough(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.openErr = oidc.SignInRefused{}
	_, err := world.logins().Switch("key-1", "example.com")
	isError(t, err, oidc.SignInRefused{})
	same(t, "events", world.events, []string{"open key-1"})
}

func TestSwitchPassesADirectoryFailureThrough(t *testing.T) {
	world := newLoginWorld(t)
	world.people.err = oidc.SessionUnavailable{}
	_, err := world.logins().Switch("key-1", "example.com")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "events", world.events, []string{"open key-1", "person person-1"})
}

func TestSignOutUsesTheSealedRefreshToken(t *testing.T) {
	world := newLoginWorld(t)
	err := world.logins().SignOut("key-1")
	failOn(t, err)
	same(t, "events", world.events, []string{"open key-1", "sign-out refresh-1"})
}

func TestSignOutRefusesABadKey(t *testing.T) {
	world := newLoginWorld(t)
	world.realm.openErr = oidc.SignInRefused{}
	err := world.logins().SignOut("forged")
	isError(t, err, oidc.SignInRefused{})
	same(t, "signed out", world.realm.signedOut, []string(nil))
}

func TestReopenedGrantsTheGivenDomain(t *testing.T) {
	world := newLoginWorld(t)
	grant, err := world.logins().Reopened(anaSession(), "example.com")
	failOn(t, err)
	same(t, "events", world.events, []string{"person person-1", "seal"})
	same(t, "domain", grant.DomainId, "example.com")
	same(t, "name", grant.DisplayName, "Ana Example")
}

func TestReopenedPassesADirectoryFailureThrough(t *testing.T) {
	world := newLoginWorld(t)
	world.people.err = oidc.SessionUnavailable{}
	_, err := world.logins().Reopened(anaSession(), "example.com")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "sealed", world.realm.sealed, []oidc.SessionContent(nil))
}
