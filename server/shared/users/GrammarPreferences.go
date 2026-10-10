import (
	"fmt"
	"regexp"
)

// GrammarPreferences keeps the grammar checker settings of a user.
type GrammarPreferences struct{}

// Normalized answers grammarCheckEnabled, grammarCheckLanguage and
// grammarCheckDisabledRules with safe defaults.
func (preferences GrammarPreferences) Normalized(value any) map[string]any {
	input := PreferenceChoices{}.Object(value)
	enabled, _ := input["grammarCheckEnabled"].(bool)
	return map[string]any{
		"grammarCheckEnabled":       enabled,
		"grammarCheckLanguage":      PreferenceChoices{}.Text(input["grammarCheckLanguage"], preferences.languages(), "auto"),
		"grammarCheckDisabledRules": preferences.rules(input["grammarCheckDisabledRules"]),
	}
}

// rules answers the distinct disabled rule ids, or none when the list is too
// long or holds anything that is not a rule id.
func (preferences GrammarPreferences) rules(value any) []string {
	rules, valid := preferences.checkedRules(value)
	if !valid {
		return []string{}
	}
	return rules
}

// checkedRules answers the distinct rule ids of value, and false when it is
// not a list of at most maximumRules rule ids.
func (preferences GrammarPreferences) checkedRules(value any) ([]string, bool) {
	list, isList := RecordValues{}.List(value)
	rules := []string{}
	if !isList || len(list) > preferences.maximumRules() {
		return rules, false
	}
	pattern := regexp.MustCompile(`^[A-Z0-9_-]{1,64}$`)
	for _, item := range list {
		rule, isText := item.(string)
		if !isText || !pattern.MatchString(rule) {
			return rules, false
		}
		if !(PreferenceChoices{}).Includes(rules, rule) {
			rules = append(rules, rule)
		}
	}
	return rules, true
}

func (GrammarPreferences) maximumRules() int { return 100 }

// Checked answers why the grammar settings a caller sent are refused, or nil.
// It never coerces text or unknown rule ids.
func (preferences GrammarPreferences) Checked(body map[string]any) *PreferenceProblem {
	if raw, present := body["grammarCheckEnabled"]; present {
		if _, isFlag := raw.(bool); !isFlag {
			return &PreferenceProblem{Message: "grammarCheckEnabled must be a boolean"}
		}
	}
	if raw, present := body["grammarCheckLanguage"]; present {
		if text, isText := raw.(string); !isText || !(PreferenceChoices{}).Includes(preferences.languages(), text) {
			return &PreferenceProblem{Message: "grammarCheckLanguage must be one of: auto, en-US, en-GB"}
		}
	}
	if raw, present := body["grammarCheckDisabledRules"]; present {
		if _, valid := preferences.checkedRules(raw); !valid {
			return &PreferenceProblem{Message: fmt.Sprintf("grammarCheckDisabledRules must contain at most %d valid rule identifiers", preferences.maximumRules())}
		}
	}
	return nil
}

func (GrammarPreferences) languages() []string { return []string{"auto", "en-US", "en-GB"} }
