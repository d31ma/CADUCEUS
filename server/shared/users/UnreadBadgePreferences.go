// UnreadBadgePreferences keeps which mailboxes the unread badge counts:
// every assigned domain, or one of them.
type UnreadBadgePreferences struct{}

// Normalized answers unreadBadgeScope and unreadBadgeDomain against the
// user's current domains; a removed domain falls back to the aggregate.
func (UnreadBadgePreferences) Normalized(value any, domains any) map[string]any {
	input := PreferenceChoices{}.Object(value)
	allowed := PreferenceChoices{}.Domains(domains)
	requested := ""
	if text, isText := input["unreadBadgeDomain"].(string); isText {
		requested = Security{}.NormalizeDomain(text)
	}
	domain := ""
	if requested != "" && (PreferenceChoices{}).Includes(allowed, requested) {
		domain = requested
	} else if len(allowed) > 0 {
		domain = allowed[0]
	}
	scope := "all"
	if input["unreadBadgeScope"] == "domain" && requested != "" && requested == domain {
		scope = "domain"
	}
	return map[string]any{"unreadBadgeScope": scope, "unreadBadgeDomain": domain}
}
