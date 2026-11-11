package addresses

import "testing"

func assertLogin(t *testing.T, input string, username string, domainId string) {
	t.Helper()
	parts, err := Login{}.Parsed(input)
	assertAccepted(t, err)
	assertText(t, parts.Username, username)
	assertText(t, parts.DomainId, domainId)
}

func assertLoginRefused(t *testing.T, input string, kind string) {
	t.Helper()
	parts, err := Login{}.Parsed(input)
	assertRefusedAs(t, err, kind)
	assertText(t, parts.Username+parts.DomainId, "")
}

func TestLoginWithPrefix(t *testing.T) {
	assertLogin(t, "ana", "ana", "")
}

func TestLoginWithUppercasePrefix(t *testing.T) {
	assertLogin(t, "ANA", "ana", "")
}

func TestLoginWithSpacedPrefix(t *testing.T) {
	assertLogin(t, " ana ", "ana", "")
}

func TestLoginWithFullAddress(t *testing.T) {
	assertLogin(t, "Ana@Example.COM", "ana", "example.com")
}

func TestLoginRefusesEmptyDomain(t *testing.T) {
	assertLoginRefused(t, "ana@", NameKinds{}.Domain())
}

func TestLoginRefusesBadDomain(t *testing.T) {
	assertLoginRefused(t, "ana@-example.com", NameKinds{}.Domain())
}

func TestLoginRefusesRoleName(t *testing.T) {
	assertLoginRefused(t, "postmaster", NameKinds{}.Username())
}

func TestLoginRefusesRoleNameWithDomain(t *testing.T) {
	assertLoginRefused(t, "postmaster@example.com", NameKinds{}.Username())
}

func TestLoginRefusesSubAddress(t *testing.T) {
	assertLoginRefused(t, "ana+news@example.com", NameKinds{}.Username())
}

func TestLoginRefusesTwoDots(t *testing.T) {
	assertLoginRefused(t, "a..b", NameKinds{}.Username())
}

func TestLoginRefusesEmptyUsername(t *testing.T) {
	assertLoginRefused(t, "@example.com", NameKinds{}.Username())
}

func TestLoginRefusesSpaceBeforeAt(t *testing.T) {
	assertLoginRefused(t, "ana @example.com", NameKinds{}.Username())
}

func TestLoginRefusesTwoAtSigns(t *testing.T) {
	assertLoginRefused(t, "ana@b@example.com", NameKinds{}.Username())
}

func TestLoginRefusesNonAscii(t *testing.T) {
	assertLoginRefused(t, "ana@bücher.de", NameKinds{}.Username())
}
