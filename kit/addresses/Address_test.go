package addresses

import "testing"

func assertAddress(t *testing.T, input string, localPart string, domainId string) {
	t.Helper()
	parts, err := Address{}.Parsed(input)
	assertAccepted(t, err)
	assertText(t, parts.LocalPart, localPart)
	assertText(t, parts.DomainId, domainId)
}

func assertAddressRefused(t *testing.T, input string, kind string) {
	t.Helper()
	parts, err := Address{}.Parsed(input)
	assertRefusedAs(t, err, kind)
	assertText(t, parts.LocalPart+parts.DomainId, "")
}

func TestAddressParsedLowersBothParts(t *testing.T) {
	assertAddress(t, " Ana@Example.COM ", "ana", "example.com")
}

func TestAddressParsedAllowsRoleName(t *testing.T) {
	assertAddress(t, "postmaster@example.com", "postmaster", "example.com")
}

func TestAddressParsedRefusesMissingAt(t *testing.T) {
	assertAddressRefused(t, "ana", NameKinds{}.Address())
}

func TestAddressParsedRefusesNonAscii(t *testing.T) {
	assertAddressRefused(t, "anä@example.com", NameKinds{}.Address())
}

func TestAddressParsedRefusesEmptyLocalPart(t *testing.T) {
	assertAddressRefused(t, "@example.com", NameKinds{}.LocalPart())
}

func TestAddressParsedRefusesSubAddress(t *testing.T) {
	assertAddressRefused(t, "ana+news@example.com", NameKinds{}.LocalPart())
}

func TestAddressParsedRefusesEmptyDomain(t *testing.T) {
	assertAddressRefused(t, "ana@", NameKinds{}.Domain())
}

func TestAddressParsedRefusesWildcardDomain(t *testing.T) {
	assertAddressRefused(t, "ana@*", NameKinds{}.Domain())
}

func TestAddressNormalized(t *testing.T) {
	got, err := Address{}.Normalized("Team.Sales@Mail.Example.COM")
	assertAccepted(t, err)
	assertText(t, got, "team.sales@mail.example.com")
}

func TestAddressNormalizedRefusesMissingAt(t *testing.T) {
	got, err := Address{}.Normalized("ana")
	assertRefusedAs(t, err, NameKinds{}.Address())
	assertText(t, got, "")
}

func TestAddressPrimary(t *testing.T) {
	assertText(t, Address{}.Primary("ana", "example.com"), "ana@example.com")
}

func TestAddressSameExternalIgnoresCaseAndSpaces(t *testing.T) {
	assertTruth(t, Address{}.SameExternal(" Bob@Partner.EXAMPLE ", "bob@partner.example"), true)
}

func TestAddressSameExternalSeesDifferentAddresses(t *testing.T) {
	assertTruth(t, Address{}.SameExternal("bob@partner.example", "bobby@partner.example"), false)
}
