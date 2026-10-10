import "regexp"

// NewMailNotificationPreferences keeps which accounts notify of new mail and
// how much a notification shows. A missing domain list means every assigned
// domain; an empty list means none.
type NewMailNotificationPreferences struct{}

// Normalized answers newMailNotificationDomains,
// newMailNotificationExternalAccounts, newMailNotificationPrivacy and
// newMailNotificationActions.
func (preferences NewMailNotificationPreferences) Normalized(value any, domains any) map[string]any {
	input := PreferenceChoices{}.Object(value)
	allowed := PreferenceChoices{}.Domains(domains)
	actions, isFlag := input["newMailNotificationActions"].(bool)
	if !isFlag {
		actions = true
	}
	return map[string]any{
		"newMailNotificationDomains":          preferences.domains(input["newMailNotificationDomains"], allowed),
		"newMailNotificationExternalAccounts": preferences.externalAccounts(input["newMailNotificationExternalAccounts"]),
		"newMailNotificationPrivacy":          PreferenceChoices{}.Text(input["newMailNotificationPrivacy"], []string{"full", "sender", "private"}, "full"),
		"newMailNotificationActions":          actions,
	}
}

func (NewMailNotificationPreferences) domains(value any, allowed []string) []string {
	requested, isList := RecordValues{}.List(value)
	if !isList {
		requested, _ = RecordValues{}.List(allowed)
	}
	chosen := []string{}
	for _, entry := range requested {
		text, isText := entry.(string)
		domain := Security{}.NormalizeDomain(text)
		if isText && domain != "" && (PreferenceChoices{}).Includes(allowed, domain) && !(PreferenceChoices{}).Includes(chosen, domain) {
			chosen = append(chosen, domain)
		}
	}
	return chosen
}

func (NewMailNotificationPreferences) externalAccounts(value any) []string {
	requested, _ := RecordValues{}.List(value)
	pattern := regexp.MustCompile(`^ext_[A-Za-z0-9_-]{20,80}$`)
	chosen := []string{}
	for _, entry := range requested {
		id, isText := entry.(string)
		if isText && pattern.MatchString(id) && !(PreferenceChoices{}).Includes(chosen, id) {
			chosen = append(chosen, id)
		}
	}
	return chosen
}
