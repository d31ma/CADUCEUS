package addresses

import (
	"slices"
	"strings"
)

type RoleNames struct{}

func (RoleNames) All() []string {
	return []string{"postmaster", "abuse", "hostmaster", "webmaster", "mailer-daemon", "noreply", "no-reply", "security"}
}

func (names RoleNames) Contains(localPart string) bool {
	return slices.Contains(names.All(), strings.ToLower(strings.TrimSpace(localPart)))
}
