import (
	"slices"
	"strings"
)

// UnreadBadgeUtils is the Go form of normalizeUnreadBadgePreference in
// shared/scripts/unread-badge.js: a focused domain that the account no
// longer holds becomes the aggregate badge.
type UnreadBadgeUtils struct{}

func (utils UnreadBadgeUtils) Normalize(scope string, domain string, domains []string) UnreadBadgePreference {
	allowed := PreferenceUtils{}.Unique(domains)
	requested := PreferenceUtils{}.Lower(domain)
	chosen := utils.domain(requested, allowed)
	return UnreadBadgePreference{Scope: utils.scope(scope, requested, chosen), Domain: chosen}
}

func (UnreadBadgeUtils) domain(requested string, allowed []string) string {
	if slices.Contains(allowed, requested) {
		return requested
	}
	return strings.Join(allowed[:min(1, len(allowed))], "")
}

func (UnreadBadgeUtils) scope(scope string, requested string, chosen string) string {
	if scope == "domain" && requested == chosen {
		return scope
	}
	return "all"
}
