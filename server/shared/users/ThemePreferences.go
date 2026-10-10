// ThemePreferences keeps the active, light and dark theme identifiers.
type ThemePreferences struct{}

// Normalized answers the three theme choices, each from its catalog.
func (ThemePreferences) Normalized(value any) map[string]any {
	input := PreferenceChoices{}.Object(value)
	choices := PreferenceChoices{}
	return map[string]any{
		"activeTheme": choices.Text(input["activeTheme"], []string{"automatic", "light", "dark", "darkside", "less-is-more", "taiga", "ubuntu"}, "automatic"),
		"lightTheme":  choices.Text(input["lightTheme"], []string{"light", "less-is-more", "taiga", "ubuntu"}, "light"),
		"darkTheme":   choices.Text(input["darkTheme"], []string{"dark", "darkside"}, "dark"),
	}
}
