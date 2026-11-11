package login

import (
	"testing"

	"mesail.dev/packages/oidc"
)

func unavailableLegacy() LegacySessions {
	issuer := oidc.SessionIssuer{Problem: oidc.NotConfigured{}}
	return LegacySessions{Issuer: issuer, Accounts: oidc.AccountManager{Sessions: issuer}}
}

func legacyGrant(roles []string) oidc.SessionGrant {
	caller := oidc.Caller{DomainId: "example.com", MemberId: "person-1", Username: "ana", DisplayName: "Ana Example", Email: "ana@example.com", Roles: roles}
	return oidc.SessionGrant{Caller: caller, AccessToken: "access", ExpiresIn: 300, SessionKey: "sealed-key"}
}

func TestLegacySessionsMapsAnAdminGrant(t *testing.T) {
	grant := LegacySessions{}.granted(legacyGrant([]string{"Member", "DomainAdmin"}))
	same(t, "grant", grant, Grant{
		PersonId:       "person-1",
		DomainId:       "example.com",
		Username:       "ana",
		DisplayName:    "Ana Example",
		PrimaryAddress: "ana@example.com",
		Roles:          []string{"Member", "DomainAdmin"},
		Domains:        []DomainChoice{{DomainId: "example.com", Admin: true, Default: true}},
		AccessToken:    "access",
		ExpiresIn:      300,
		SessionKey:     "sealed-key",
	})
}

func TestLegacySessionsMapsAMemberGrant(t *testing.T) {
	grant := LegacySessions{}.granted(legacyGrant([]string{"Member"}))
	same(t, "domains", grant.Domains, []DomainChoice{{DomainId: "example.com", Admin: false, Default: true}})
}

func TestLegacySessionsNeverSwitches(t *testing.T) {
	_, err := unavailableLegacy().Switch("key-1", "example.com")
	isError(t, err, NotFound{})
}

func TestLegacySessionsSignInPassesAFailureThrough(t *testing.T) {
	grant, err := unavailableLegacy().SignIn("ana@example.com", "right-password", "")
	isError(t, err, oidc.SessionUnavailable{})
	same(t, "domains", grant.Domains, []DomainChoice{{DomainId: "", Admin: false, Default: true}})
}

func TestLegacySessionsRefreshPassesAFailureThrough(t *testing.T) {
	_, err := unavailableLegacy().Refresh("key-1")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestLegacySessionsSignOutPassesAFailureThrough(t *testing.T) {
	err := unavailableLegacy().SignOut("key-1")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestLegacySessionsIdentifyRefusesABadHeader(t *testing.T) {
	_, err := unavailableLegacy().Identify("")
	isError(t, err, oidc.MissingBearerToken{})
}

func TestLegacySessionsChangePasswordPassesAFailureThrough(t *testing.T) {
	_, err := unavailableLegacy().ChangePassword("Bearer token", "right-password", "a-new-long-password", "")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestLegacySessionsTotpEnabledPassesAFailureThrough(t *testing.T) {
	_, err := unavailableLegacy().TotpEnabled("Bearer token")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestLegacySessionsSetupTotpPassesAFailureThrough(t *testing.T) {
	_, err := unavailableLegacy().SetupTotp("Bearer token", "right-password")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestLegacySessionsConfirmTotpPassesAFailureThrough(t *testing.T) {
	err := unavailableLegacy().ConfirmTotp("Bearer token", "setup-key", "123456")
	isError(t, err, oidc.SessionUnavailable{})
}

func TestLegacySessionsRemoveTotpPassesAFailureThrough(t *testing.T) {
	err := unavailableLegacy().RemoveTotp("Bearer token", "right-password", "123456")
	isError(t, err, oidc.SessionUnavailable{})
}
