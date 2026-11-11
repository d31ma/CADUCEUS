package addresses

import (
	"strings"
	"testing"
)

func assertLocalPart(t *testing.T, input string, want string) {
	t.Helper()
	got, err := LocalPart{}.Normalized(input)
	assertAccepted(t, err)
	assertText(t, got, want)
}

func assertLocalPartRefused(t *testing.T, input string) {
	t.Helper()
	got, err := LocalPart{}.Normalized(input)
	assertRefusedAs(t, err, NameKinds{}.LocalPart())
	assertText(t, got, "")
}

func TestLocalPartTrimsAndLowers(t *testing.T) {
	assertLocalPart(t, " Ana.B_c-1 ", "ana.b_c-1")
}

func TestLocalPartAllowsRoleName(t *testing.T) {
	assertLocalPart(t, "Postmaster", "postmaster")
}

func TestLocalPartAllowsOneCharacter(t *testing.T) {
	assertLocalPart(t, "7", "7")
}

func TestLocalPartAllowsSixtyFourCharacters(t *testing.T) {
	assertLocalPart(t, strings.Repeat("a", 64), strings.Repeat("a", 64))
}

func TestLocalPartRefusesSixtyFiveCharacters(t *testing.T) {
	assertLocalPartRefused(t, strings.Repeat("a", 65))
}

func TestLocalPartRefusesEmpty(t *testing.T) {
	assertLocalPartRefused(t, "   ")
}

func TestLocalPartRefusesNonAscii(t *testing.T) {
	assertLocalPartRefused(t, "anä")
}

func TestLocalPartRefusesKelvinSign(t *testing.T) {
	assertLocalPartRefused(t, "Kim")
}

func TestLocalPartRefusesPlus(t *testing.T) {
	assertLocalPartRefused(t, "ana+news")
}

func TestLocalPartRefusesAt(t *testing.T) {
	assertLocalPartRefused(t, "ana@example.com")
}

func TestLocalPartRefusesInnerSpace(t *testing.T) {
	assertLocalPartRefused(t, "ana b")
}

func TestLocalPartRefusesLeadingDot(t *testing.T) {
	assertLocalPartRefused(t, ".ana")
}

func TestLocalPartRefusesTrailingDash(t *testing.T) {
	assertLocalPartRefused(t, "ana-")
}

func TestLocalPartRefusesLeadingUnderscore(t *testing.T) {
	assertLocalPartRefused(t, "_ana")
}

func TestLocalPartRefusesTwoDotsInARow(t *testing.T) {
	assertLocalPartRefused(t, "a..b")
}
