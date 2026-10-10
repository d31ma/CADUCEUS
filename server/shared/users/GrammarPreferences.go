import "regexp"

// GrammarPreferences keeps the grammar checker settings of a user.
type GrammarPreferences struct{}

// Normalized answers grammarCheckEnabled, grammarCheckLanguage and
// grammarCheckDisabledRules with safe defaults.
func (preferences GrammarPreferences) Normalized(value any) map[string]any {
	input := PreferenceChoices{}.Object(value)
	enabled, _ := input["grammarCheckEnabled"].(bool)
	return map[string]any{
		"grammarCheckEnabled":       enabled,
		"grammarCheckLanguage":      PreferenceChoices{}.Text(input["grammarCheckLanguage"], []string{"auto", "en-US", "en-GB"}, "auto"),
		"grammarCheckDisabledRules": preferences.rules(input["grammarCheckDisabledRules"]),
	}
}

// rules answers the distinct disabled rule ids, or none when the list is too
// long or holds anything that is not a rule id.
func (preferences GrammarPreferences) rules(value any) []string {
	list, isList := RecordValues{}.List(value)
	rules := []string{}
	if !isList || len(list) > preferences.maximumRules() {
		return rules
	}
	pattern := regexp.MustCompile(`^[A-Z0-9_-]{1,64}$`)
	for _, item := range list {
		rule, isText := item.(string)
		if !isText || !pattern.MatchString(rule) {
			return []string{}
		}
		if !(PreferenceChoices{}).Includes(rules, rule) {
			rules = append(rules, rule)
		}
	}
	return rules
}

func (GrammarPreferences) maximumRules() int { return 100 }
