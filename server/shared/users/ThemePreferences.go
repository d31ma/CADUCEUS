import "strings"

// ThemePreferences keeps the active, light and dark theme identifiers.
type ThemePreferences struct{}

// Normalized answers the three theme choices, each from its catalog.
func (preferences ThemePreferences) Normalized(value any) map[string]any {
	input := PreferenceChoices{}.Object(value)
	choices := PreferenceChoices{}
	catalogs := preferences.catalogs()
	return map[string]any{
		"activeTheme": choices.Text(input["activeTheme"], catalogs["activeTheme"], "automatic"),
		"lightTheme":  choices.Text(input["lightTheme"], catalogs["lightTheme"], "light"),
		"darkTheme":   choices.Text(input["darkTheme"], catalogs["darkTheme"], "dark"),
	}
}

func (ThemePreferences) catalogs() map[string][]string {
	return map[string][]string{
		"activeTheme": {"automatic", "light", "dark", "darkside", "less-is-more", "taiga", "ubuntu"},
		"lightTheme":  {"light", "less-is-more", "taiga", "ubuntu"},
		"darkTheme":   {"dark", "darkside"},
	}
}

// Checked answers the theme choices a caller sent, or why they are refused:
// only catalog identifiers are kept, never CSS, paths or package names.
func (preferences ThemePreferences) Checked(value any) (map[string]any, *PreferenceProblem) {
	input, isObject := RecordValues{}.Object(value)
	if !isObject {
		return nil, &PreferenceProblem{Message: "themePreferences must be an object"}
	}
	for _, key := range (PreferenceChoices{}).Keys(input) {
		if _, known := preferences.catalogs()[key]; !known {
			return nil, &PreferenceProblem{Message: "Unknown theme preference: " + key}
		}
	}
	for _, key := range []string{"activeTheme", "lightTheme", "darkTheme"} {
		raw, present := input[key]
		text, isText := raw.(string)
		if present && !(isText && (PreferenceChoices{}).Includes(preferences.catalogs()[key], text)) {
			return nil, &PreferenceProblem{Message: key + " must be one of: " + strings.Join(preferences.catalogs()[key], ", ")}
		}
	}
	return preferences.Normalized(input), nil
}
