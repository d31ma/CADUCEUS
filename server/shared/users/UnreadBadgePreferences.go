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

// Checked answers the badge scope a caller's patch leaves, or why it is
// refused: the caller may only name a domain assigned to the user.
func (preferences UnreadBadgePreferences) Checked(patch map[string]any, current any, domains any) (map[string]any, *PreferenceProblem) {
	allowed := PreferenceChoices{}.Domains(domains)
	existing := preferences.Normalized(current, allowed)
	rawScope, scopeNamed := patch["unreadBadgeScope"]
	if scope, isText := rawScope.(string); scopeNamed && !(isText && (scope == "all" || scope == "domain")) {
		return nil, &PreferenceProblem{Message: "unreadBadgeScope must be one of: all, domain"}
	}
	domain := existing["unreadBadgeDomain"]
	if raw, named := patch["unreadBadgeDomain"]; named {
		text, _ := raw.(string)
		normalized := Security{}.NormalizeDomain(text)
		if normalized == "" {
			return nil, &PreferenceProblem{Message: "unreadBadgeDomain must be an assigned domain"}
		}
		if !(PreferenceChoices{}).Includes(allowed, normalized) {
			return nil, &PreferenceProblem{Message: "Unread badge domain is not assigned to this user", Forbidden: true}
		}
		domain = normalized
	}
	scope := existing["unreadBadgeScope"]
	if scopeNamed {
		scope = rawScope
	}
	if scope == "domain" && domain == "" {
		return nil, &PreferenceProblem{Message: "unreadBadgeDomain is required for domain scope"}
	}
	return map[string]any{"unreadBadgeScope": scope, "unreadBadgeDomain": domain}, nil
}
