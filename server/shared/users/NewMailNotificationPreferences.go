import (
	"fmt"
	"regexp"
)

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

func (preferences NewMailNotificationPreferences) externalAccounts(value any) []string {
	requested, _ := RecordValues{}.List(value)
	chosen := []string{}
	for _, entry := range requested {
		id, isText := entry.(string)
		if isText && preferences.accountPattern().MatchString(id) && !(PreferenceChoices{}).Includes(chosen, id) {
			chosen = append(chosen, id)
		}
	}
	return chosen
}

func (NewMailNotificationPreferences) accountPattern() *regexp.Regexp {
	return regexp.MustCompile(`^ext_[A-Za-z0-9_-]{20,80}$`)
}

func (NewMailNotificationPreferences) maximumEntries() int { return 50 }

// ForAccounts answers the preferences when the user's connected accounts are
// known: only those accounts can notify, and a user who never chose gets all
// of them.
func (preferences NewMailNotificationPreferences) ForAccounts(value any, domains any, accounts []string) map[string]any {
	normalized := preferences.Normalized(value, domains)
	allowed := preferences.externalAccounts(accounts)
	requested, isList := RecordValues{}.List(PreferenceChoices{}.Object(value)["newMailNotificationExternalAccounts"])
	if !isList {
		requested, _ = RecordValues{}.List(allowed)
	}
	chosen := []string{}
	for _, id := range preferences.externalAccounts(requested) {
		if (PreferenceChoices{}).Includes(allowed, id) {
			chosen = append(chosen, id)
		}
	}
	normalized["newMailNotificationExternalAccounts"] = chosen
	return normalized
}

// Checked answers the preferences a caller's patch leaves, or why it is
// refused: the caller may only name domains and connected accounts assigned
// to the user.
func (preferences NewMailNotificationPreferences) Checked(patch map[string]any, current any, scope NotificationScope) (map[string]any, *PreferenceProblem) {
	allowed := PreferenceChoices{}.Domains(scope.Domains)
	accounts := preferences.externalAccounts(scope.Accounts)
	next := preferences.ForAccounts(current, allowed, accounts)
	if raw, named := patch["newMailNotificationDomains"]; named {
		chosen, problem := preferences.checkedDomains(raw, allowed)
		if problem != nil {
			return nil, problem
		}
		next["newMailNotificationDomains"] = chosen
	}
	if raw, named := patch["newMailNotificationExternalAccounts"]; named {
		chosen, problem := preferences.checkedAccounts(raw, accounts)
		if problem != nil {
			return nil, problem
		}
		next["newMailNotificationExternalAccounts"] = chosen
	}
	if raw, named := patch["newMailNotificationPrivacy"]; named {
		text, isText := raw.(string)
		if !isText || !(PreferenceChoices{}).Includes([]string{"full", "sender", "private"}, text) {
			return nil, &PreferenceProblem{Message: "newMailNotificationPrivacy must be one of: full, sender, private"}
		}
		next["newMailNotificationPrivacy"] = text
	}
	if raw, named := patch["newMailNotificationActions"]; named {
		flag, isFlag := raw.(bool)
		if !isFlag {
			return nil, &PreferenceProblem{Message: "newMailNotificationActions must be a boolean"}
		}
		next["newMailNotificationActions"] = flag
	}
	return next, nil
}

func (preferences NewMailNotificationPreferences) checkedDomains(value any, allowed []string) ([]string, *PreferenceProblem) {
	list, problem := preferences.bounded(value, "newMailNotificationDomains")
	if problem != nil {
		return nil, problem
	}
	chosen := []string{}
	for _, entry := range list {
		text, _ := entry.(string)
		domain := Security{}.NormalizeDomain(text)
		if domain == "" {
			return nil, &PreferenceProblem{Message: "Each new-mail notification account must be a domain"}
		}
		if !(PreferenceChoices{}).Includes(allowed, domain) {
			return nil, &PreferenceProblem{Message: "Notification domain is not assigned to this user", Forbidden: true}
		}
		if !(PreferenceChoices{}).Includes(chosen, domain) {
			chosen = append(chosen, domain)
		}
	}
	return chosen, nil
}

func (preferences NewMailNotificationPreferences) checkedAccounts(value any, allowed []string) ([]string, *PreferenceProblem) {
	list, problem := preferences.bounded(value, "newMailNotificationExternalAccounts")
	if problem != nil {
		return nil, problem
	}
	chosen := []string{}
	for _, entry := range list {
		id, _ := entry.(string)
		if !preferences.accountPattern().MatchString(id) {
			return nil, &PreferenceProblem{Message: "Each connected new-mail notification account must be an external account id"}
		}
		if !(PreferenceChoices{}).Includes(allowed, id) {
			return nil, &PreferenceProblem{Message: "Notification account is not assigned to this user", Forbidden: true}
		}
		if !(PreferenceChoices{}).Includes(chosen, id) {
			chosen = append(chosen, id)
		}
	}
	return chosen, nil
}

// bounded answers the list a patch field holds, or why it is not a list of
// at most maximumEntries accounts.
func (preferences NewMailNotificationPreferences) bounded(value any, field string) ([]any, *PreferenceProblem) {
	list, isList := RecordValues{}.List(value)
	if !isList {
		return nil, &PreferenceProblem{Message: field + " must be an array"}
	}
	if len(list) > preferences.maximumEntries() {
		return nil, &PreferenceProblem{Message: fmt.Sprintf("%s may contain at most %d accounts", field, preferences.maximumEntries())}
	}
	return list, nil
}
