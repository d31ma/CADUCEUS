package addresses

import "testing"

func assertUsername(t *testing.T, input string, want string) {
	t.Helper()
	got, err := Username{}.Normalized(input)
	assertAccepted(t, err)
	assertText(t, got, want)
}

func assertUsernameRefused(t *testing.T, input string) {
	t.Helper()
	got, err := Username{}.Normalized(input)
	assertRefusedAs(t, err, NameKinds{}.Username())
	assertText(t, got, "")
}

func TestUsernameKeepsLowercase(t *testing.T) {
	assertUsername(t, "ana", "ana")
}

func TestUsernameLowersUppercase(t *testing.T) {
	assertUsername(t, "ANA", "ana")
}

func TestUsernameTrimsSpaces(t *testing.T) {
	assertUsername(t, " ana ", "ana")
}

func TestUsernameRefusesPostmaster(t *testing.T) {
	assertUsernameRefused(t, "postmaster")
}

func TestUsernameRefusesNoReplyInAnyCase(t *testing.T) {
	assertUsernameRefused(t, "No-Reply")
}

func TestUsernameRefusesPlus(t *testing.T) {
	assertUsernameRefused(t, "ana+news")
}

func TestUsernameRefusesTwoDots(t *testing.T) {
	assertUsernameRefused(t, "a..b")
}

func TestUsernameRefusesNonAscii(t *testing.T) {
	assertUsernameRefused(t, "jörg")
}
