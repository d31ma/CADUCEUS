package addresses

import (
	"strings"
	"testing"
)

func TestRoleNamesListsEveryRole(t *testing.T) {
	assertText(t, strings.Join(RoleNames{}.All(), ","), "postmaster,abuse,hostmaster,webmaster,mailer-daemon,noreply,no-reply,security")
}

func TestRoleNamesContainsIgnoresCaseAndSpaces(t *testing.T) {
	assertTruth(t, RoleNames{}.Contains(" Mailer-Daemon "), true)
}

func TestRoleNamesDoesNotContainPersonName(t *testing.T) {
	assertTruth(t, RoleNames{}.Contains("ana"), false)
}
